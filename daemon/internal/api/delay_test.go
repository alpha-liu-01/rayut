package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"github.com/alpha-liu-01/rayut/daemon/internal/profile"
)

func TestSingleDelayReturnsBeforeTheProbe(t *testing.T) {
	data := t.TempDir()
	oldProfile, oldRuntime := paths.Profile, paths.Runtime
	oldUp, oldMeasure := coreUp, measureDelay
	paths.Profile = filepath.Join(data, "active.yaml")
	paths.Runtime = filepath.Join(data, "runtime")
	coreUp = func() bool { return true }
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var calls int
	var mu sync.Mutex
	measureDelay = func(context.Context, string) (int, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		once.Do(func() { close(entered) })
		<-release
		return 42, nil
	}
	t.Cleanup(func() {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		default:
			close(release)
		}
		paths.Profile, paths.Runtime = oldProfile, oldRuntime
		coreUp, measureDelay = oldUp, oldMeasure
	})

	store := &profile.Store{Dir: data, Test: func(string) error { return nil }}
	groups, err := store.ImportText("甲", "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 1}\n")
	if err != nil || len(groups) != 1 || !groups[0].Active {
		t.Fatalf("import %+v %v", groups, err)
	}
	id := groups[0].ID
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/v1/groups/"+id+"/delay", strings.NewReader(`{"name":"alpha"}`))
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		s.groupDelay(rec, req, id)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delay start blocked")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"running":true`) || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	again := httptest.NewRecorder()
	s.groupDelay(again, httptest.NewRequest(http.MethodPost, "/v1/groups/"+id+"/delay", strings.NewReader(`{"name":"alpha"}`)), id)
	mu.Lock()
	started := calls
	mu.Unlock()
	if again.Code != http.StatusOK || started != 1 {
		t.Fatalf("second start %d calls %d", again.Code, started)
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		progress := httptest.NewRecorder()
		s.delayProgress(progress, id)
		if strings.Contains(progress.Body.String(), `"running":false`) && strings.Contains(progress.Body.String(), `"done":1`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("progress %s", progress.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	nodes, err := store.Nodes(id)
	if err != nil || len(nodes) != 1 || nodes[0].Name != "alpha" || nodes[0].Delay != 42 {
		t.Fatalf("stored %+v %v", nodes, err)
	}
}

func TestFailedDelayIsStoredWithoutBlocking(t *testing.T) {
	data := t.TempDir()
	oldProfile, oldRuntime := paths.Profile, paths.Runtime
	oldUp, oldMeasure := coreUp, measureDelay
	paths.Profile = filepath.Join(data, "active.yaml")
	paths.Runtime = filepath.Join(data, "runtime")
	coreUp = func() bool { return true }
	measureDelay = func(context.Context, string) (int, error) {
		return 0, context.DeadlineExceeded
	}
	t.Cleanup(func() {
		paths.Profile, paths.Runtime = oldProfile, oldRuntime
		coreUp, measureDelay = oldUp, oldMeasure
	})
	store := &profile.Store{Dir: data, Test: func(string) error { return nil }}
	groups, err := store.ImportText("甲", "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 1}\n")
	if err != nil || len(groups) != 1 {
		t.Fatalf("import %+v %v", groups, err)
	}
	s := &Server{}
	rec := httptest.NewRecorder()
	s.groupDelay(rec, httptest.NewRequest(http.MethodPost, "/delay", strings.NewReader(`{"name":"alpha"}`)), groups[0].ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		nodes, err := store.Nodes(groups[0].ID)
		if err == nil && len(nodes) == 1 && nodes[0].Delay == -1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stored %+v %v", nodes, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDelayStartRejectsEmptyNameAndIdleCore(t *testing.T) {
	oldUp, oldMeasure := coreUp, measureDelay
	called := false
	coreUp = func() bool { return false }
	measureDelay = func(context.Context, string) (int, error) {
		called = true
		return 1, nil
	}
	t.Cleanup(func() {
		coreUp, measureDelay = oldUp, oldMeasure
	})
	data := t.TempDir()
	oldProfile, oldRuntime := paths.Profile, paths.Runtime
	paths.Profile = filepath.Join(data, "active.yaml")
	paths.Runtime = filepath.Join(data, "runtime")
	t.Cleanup(func() {
		paths.Profile, paths.Runtime = oldProfile, oldRuntime
	})
	store := &profile.Store{Dir: data, Test: func(string) error { return nil }}
	groups, err := store.ImportText("甲", "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 1}\n")
	if err != nil || len(groups) != 1 {
		t.Fatalf("import %+v %v", groups, err)
	}
	s := &Server{}
	empty := httptest.NewRecorder()
	s.groupDelay(empty, httptest.NewRequest(http.MethodPost, "/delay", strings.NewReader(`{"name":"  "}`)), groups[0].ID)
	if empty.Code != http.StatusBadRequest || strings.TrimSpace(empty.Body.String()) != "invalid name" || called {
		t.Fatalf("empty %d %q called %v", empty.Code, empty.Body.String(), called)
	}
	idle := httptest.NewRecorder()
	s.groupDelay(idle, httptest.NewRequest(http.MethodPost, "/delay", strings.NewReader(`{"name":"alpha"}`)), groups[0].ID)
	if idle.Code != http.StatusConflict || strings.TrimSpace(idle.Body.String()) != "core not running" || called {
		t.Fatalf("idle %d %q called %v", idle.Code, idle.Body.String(), called)
	}
}
