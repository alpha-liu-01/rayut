package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

func TestGroupRoutesStayOnLoopbackShape(t *testing.T) {
	dir := t.TempDir()
	old := paths.Profile
	paths.Profile = filepath.Join(dir, "profiles", "active.yaml")
	t.Cleanup(func() { paths.Profile = old })
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/v1/groups", nil)
	rec := httptest.NewRecorder()
	s.groupsRoot(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"groups":[]`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	missing := httptest.NewRequest(http.MethodPost, "/v1/groups/abcd1234/use", nil)
	missingRec := httptest.NewRecorder()
	s.groupItem(missingRec, missing)
	if missingRec.Code != http.StatusBadRequest || !strings.Contains(missingRec.Body.String(), "group missing") {
		t.Fatalf("missing %d %s", missingRec.Code, missingRec.Body.String())
	}
}
