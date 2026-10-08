package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/mihomoapi"
)

type fakeGroups struct {
	group string
	name  string
	delay string
}

func (f *fakeGroups) Groups(context.Context) ([]mihomoapi.Group, error) {
	return []mihomoapi.Group{{
		Name:       f.group,
		Type:       "Selector",
		Now:        f.name,
		Selectable: true,
		Nodes:      []mihomoapi.Node{{Name: f.name}},
	}}, nil
}

func (f *fakeGroups) Select(_ context.Context, group, name string) error {
	f.group = group
	f.name = name
	return nil
}

func (f *fakeGroups) Delay(_ context.Context, name string) (int, error) {
	f.delay = name
	if name == "down" {
		return 0, mihomoapi.ErrTimeout
	}
	return 15, nil
}

func TestSelectionKeepsSpecialNames(t *testing.T) {
	fake := &fakeGroups{}
	s := &Server{groups: fake}
	group := "组;$(rm)"
	node := "节点\"$/"
	req := httptest.NewRequest(http.MethodPut, "/v1/proxy-groups/"+url.PathEscape(group)+"/selection", strings.NewReader(`{"name":"节点\"$/"}`))
	rec := httptest.NewRecorder()
	s.proxyGroupSelection(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if fake.group != group || fake.name != node {
		t.Fatalf("got group %q name %q", fake.group, fake.name)
	}
	if strings.Contains(rec.Body.String(), "uuid") || strings.Contains(rec.Body.String(), "password") {
		t.Fatal(rec.Body.String())
	}
}

func TestDelayErrorStaysOnRayut(t *testing.T) {
	s := &Server{groups: &fakeGroups{}}
	req := httptest.NewRequest(http.MethodPost, "/v1/proxies/"+url.PathEscape("down")+"/delay", nil)
	rec := httptest.NewRecorder()
	s.proxyDelay(rec, req)
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "timeout" {
		t.Fatalf("body %q", rec.Body.String())
	}
}
