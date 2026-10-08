package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/mihomoapi"
)

type groupAPI interface {
	Groups(context.Context) ([]mihomoapi.Group, error)
	Select(context.Context, string, string) error
	Delay(context.Context, string) (int, error)
}

func (s *Server) proxyAPI() (groupAPI, error) {
	if s.groups != nil {
		return s.groups, nil
	}
	if _, ok := core.Alive(); !ok {
		return nil, mihomoapi.ErrCoreNotRunning
	}
	return mihomoapi.Default()
}

func (s *Server) proxyGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/v1/proxy-groups" {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	api, err := s.proxyAPI()
	if err != nil {
		writeProxyError(w, err)
		return
	}
	groups, err := api.Groups(r.Context())
	if err != nil {
		writeProxyError(w, err)
		return
	}
	writeJSON(w, map[string]any{"groups": groups})
}

func (s *Server) proxyGroupSelection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	group, err := pathSegment(r.URL.EscapedPath(), "/v1/proxy-groups/", "/selection")
	if err != nil {
		writeProxyError(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	api, err := s.proxyAPI()
	if err != nil {
		writeProxyError(w, err)
		return
	}
	if err := api.Select(r.Context(), group, body.Name); err != nil {
		writeProxyError(w, err)
		return
	}
	groups, err := api.Groups(r.Context())
	if err != nil {
		writeProxyError(w, err)
		return
	}
	writeJSON(w, map[string]any{"groups": groups})
}

func (s *Server) proxyDelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	name, err := pathSegment(r.URL.EscapedPath(), "/v1/proxies/", "/delay")
	if err != nil {
		writeProxyError(w, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	api, err := s.proxyAPI()
	if err != nil {
		writeProxyError(w, err)
		return
	}
	delay, err := api.Delay(r.Context(), name)
	if err != nil {
		writeProxyError(w, err)
		return
	}
	writeJSON(w, map[string]int{"delay": delay})
}

func pathSegment(escapedPath, prefix, suffix string) (string, error) {
	if !strings.HasPrefix(escapedPath, prefix) || !strings.HasSuffix(escapedPath, suffix) {
		return "", mihomoapi.ErrName
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(escapedPath, prefix), suffix)
	if mid == "" || strings.Contains(mid, "/") {
		return "", mihomoapi.ErrName
	}
	name, err := url.PathUnescape(mid)
	if err != nil || name == "" {
		return "", mihomoapi.ErrName
	}
	return name, nil
}

func writeProxyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mihomoapi.ErrCoreNotRunning):
		http.Error(w, "core not running", http.StatusConflict)
	case errors.Is(err, mihomoapi.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, mihomoapi.ErrNotSelectable):
		http.Error(w, "not selectable", http.StatusBadRequest)
	case errors.Is(err, mihomoapi.ErrName):
		http.Error(w, "invalid name", http.StatusBadRequest)
	case errors.Is(err, mihomoapi.ErrTimeout):
		http.Error(w, "timeout", http.StatusGatewayTimeout)
	case errors.Is(err, mihomoapi.ErrDelay):
		http.Error(w, "delay failed", http.StatusServiceUnavailable)
	case errors.Is(err, mihomoapi.ErrRejected):
		http.Error(w, "selection rejected", http.StatusBadRequest)
	default:
		http.Error(w, "controller unavailable", http.StatusServiceUnavailable)
	}
}
