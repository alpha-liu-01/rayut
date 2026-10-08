package profile

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInvalidImportKeepsActive(t *testing.T) {
	dir := t.TempDir()
	active := []byte("original: true\n")
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error { return nil }}
	if _, err := store.ImportContent("本地", "allow-lan: true\n"); err == nil {
		t.Fatal("expected rejection")
	}
	got, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(active) {
		t.Fatalf("active changed: %s", got)
	}
	good, err := os.ReadFile(filepath.Join(dir, "last-good.yaml"))
	if err != nil || string(good) != string(active) {
		t.Fatalf("last good = %s %v", good, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "candidate.yaml")); !os.IsNotExist(err) {
		t.Fatal("candidate should not remain")
	}
}

func TestActivateOnlyAfterValidation(t *testing.T) {
	dir := t.TempDir()
	active := []byte("original: true\n")
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	tested := false
	store := &Store{Dir: dir, Test: func(path string) error {
		body, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(body), "stack: gvisor") {
			t.Fatalf("tested file = %s %v", body, err)
		}
		tested = true
		return nil
	}}
	view, err := store.ImportContent("本地", "proxies: []\n")
	if err != nil {
		t.Fatal(err)
	}
	if view.Candidate.State != "validated" || view.Current.State != "current" {
		t.Fatalf("view %+v", view)
	}
	still, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if string(still) != string(active) {
		t.Fatal("import replaced active")
	}
	if !tested {
		t.Fatal("config was not tested")
	}
	if _, err := store.Activate(); err != nil {
		t.Fatal(err)
	}
	next, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if !strings.Contains(string(next), "stack: gvisor") {
		t.Fatalf("activated file = %s", next)
	}
}

func TestTunRunningRefusesActivate(t *testing.T) {
	dir := t.TempDir()
	store := &Store{Dir: dir, Test: func(string) error { return nil }, TunUp: func() bool { return true }}
	if _, err := store.ImportContent("本地", "proxies: []\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Activate(); codeOf(err) != "tun running" {
		t.Fatal(err)
	}
}

func TestSubscriptionSnapshotOmitsQuery(t *testing.T) {
	token := "secret-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != token {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer server.Close()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte("original: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{
		Dir:           dir,
		AllowLoopback: true,
		Test:          func(string) error { return nil },
		Client: &http.Client{
			Timeout:       2 * time.Second,
			Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
			CheckRedirect: redirectPolicy(true),
		},
	}
	raw := server.URL + "/sub?token=" + token
	view, err := store.ImportURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(view.Candidate.Host, token) || strings.Contains(view.Candidate.Name, token) {
		t.Fatalf("snapshot leaked token: %+v", view)
	}
	state, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if strings.Contains(string(state), token) {
		t.Fatalf("state leaked token: %s", state)
	}
	still, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if string(still) != "original: true\n" {
		t.Fatal("url import replaced active")
	}
}

func TestRefreshTimeoutKeepsActive(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(time.Second)
	}))
	defer server.Close()
	dir := t.TempDir()
	original := []byte("original: true\n")
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subscription.url"), []byte(server.URL+"/sub\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{
		Dir:           dir,
		AllowLoopback: true,
		Test: func(string) error {
			t.Fatal("timeout should not validate")
			return nil
		},
		Client: &http.Client{
			Timeout:       100 * time.Millisecond,
			Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
			CheckRedirect: redirectPolicy(true),
		},
	}
	if _, err := store.Refresh(); codeOf(err) != "fetch failed" {
		t.Fatal(err)
	}
	<-started
	got, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if string(got) != string(original) {
		t.Fatalf("active changed: %s", got)
	}
}

func TestDownloadSendsClashUserAgentAndKeepsGroups(t *testing.T) {
	var got string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("proxies:\n- {name: a, type: socks5, server: 127.0.0.1, port: 1}\nproxy-groups:\n- {name: 手动, type: select, proxies: [a]}\n- {name: 自动, type: url-test, proxies: [a]}\nrules: [MATCH,手动]\n"))
	}))
	defer server.Close()
	dir := t.TempDir()
	store := &Store{
		Dir:           dir,
		AllowLoopback: true,
		Test:          func(string) error { return nil },
		Client: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}
	if _, err := store.ImportURL(server.URL); err != nil {
		t.Fatal(err)
	}
	if got != subscriptionUserAgent {
		t.Fatalf("user agent %q", got)
	}
	body, err := os.ReadFile(filepath.Join(dir, "candidate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "手动") || !strings.Contains(text, "自动") || strings.Contains(text, "name: Rayut") {
		t.Fatalf("candidate flattened groups: %s", text)
	}
}
