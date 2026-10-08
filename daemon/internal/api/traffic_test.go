package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"github.com/alpha-liu-01/rayut/daemon/internal/traffic"
)

func TestTrafficHandlerKeepsCumulativeAndOmitsSecrets(t *testing.T) {
	dir := t.TempDir()
	old := paths.Profile
	paths.Profile = filepath.Join(dir, "profiles", "active.yaml")
	t.Cleanup(func() { paths.Profile = old })
	path := traffic.Path()
	const planted = `{"upload":40,"download":80,"sessionUpload":0,"sessionDownload":0,"password":"super-secret-password","host":"example.com"}`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(planted), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Server{traffic: traffic.New(path)}
	req := httptest.NewRequest(http.MethodGet, "/v1/traffic", nil)
	rec := httptest.NewRecorder()
	s.trafficView(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"uploadTotal":0`, `"downloadTotal":0`, `"up":0`, `"down":0`, `"cumulativeUpload":40`, `"cumulativeDownload":80`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	if strings.Contains(body, "super-secret-password") || strings.Contains(body, "example.com") {
		t.Fatalf("body %s", body)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "super-secret-password") || strings.Contains(string(saved), "example.com") {
		t.Fatalf("record %s", saved)
	}
}
