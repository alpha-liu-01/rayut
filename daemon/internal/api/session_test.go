package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/mihomoapi"
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

func TestLogsHandlerOmitsCredentials(t *testing.T) {
	dir := t.TempDir()
	old := paths.Runtime
	paths.Runtime = dir
	t.Cleanup(func() { paths.Runtime = old })
	const sample = "time=\"2026-10-07T21:00:00.000000000-04:00\" level=info msg=\"Tun adapter listening at Meta\"\n" +
		"password: super-secret-password uuid: 11111111-2222-3333-4444-555555555555 public-key: reality-public-key private-key: reality-private-key short-id: realityshort\n" +
		"https://airport.example/sub?token=sub-token-value\n" +
		"Authorization: Bearer controller-token-value\n"
	if err := os.WriteFile(filepath.Join(dir, "mihomo.log"), []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/v1/logs", nil)
	rec := httptest.NewRecorder()
	s.logs(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{
		"super-secret-password",
		"11111111-2222-3333-4444-555555555555",
		"reality-public-key",
		"reality-private-key",
		"realityshort",
		"sub-token-value",
		"controller-token-value",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("secret %s remained", secret)
		}
	}
	if !strings.Contains(body, "Tun adapter listening at Meta") {
		t.Fatalf("body %s", body)
	}
}

type fakeSession struct {
	list []mihomoapi.Connection
}

func (f fakeSession) Connections(context.Context) ([]mihomoapi.Connection, error) {
	return f.list, nil
}

func TestConnectionsHandlerKeepsSessionFields(t *testing.T) {
	s := &Server{session: fakeSession{list: []mihomoapi.Connection{{
		Destination: "example.com:443",
		Rule:        "Domain(example.com)",
		Chain:       "节点 → Rayut",
		Upload:      4,
		Download:    10,
	}}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/connections", nil)
	rec := httptest.NewRecorder()
	s.connections(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"example.com:443", "Domain(example.com)", "节点 → Rayut", `"upload":4`, `"download":10`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestConnectionsRequireCore(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/v1/connections", nil)
	rec := httptest.NewRecorder()
	s.connections(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestControlPortRejectsLAN(t *testing.T) {
	host, _, err := net.SplitHostPort(ListenAddr)
	if err != nil || host != "127.0.0.1" {
		t.Fatal(ListenAddr)
	}
	if !strings.HasPrefix(core.ExternalController, "127.0.0.1:") {
		t.Fatal(core.ExternalController)
	}
	s := &Server{token: "t"}
	called := false
	handler := s.auth(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodGet, "/v1/logs", nil)
	req.RemoteAddr = "192.0.2.8:40000"
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if called || rec.Code != http.StatusForbidden {
		t.Fatalf("code %d called %v body %s", rec.Code, called, rec.Body.String())
	}

	loop := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	loop.RemoteAddr = "127.0.0.1:9"
	loop.Header.Set("Authorization", "Bearer t")
	loopRec := httptest.NewRecorder()
	called = false
	handler(loopRec, loop)
	if !called || loopRec.Code != http.StatusOK {
		t.Fatalf("loopback code %d called %v", loopRec.Code, called)
	}
}
