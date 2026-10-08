package api

import (
	"context"
	"net/http"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/mihomoapi"
)

type sessionAPI interface {
	Connections(context.Context) ([]mihomoapi.Connection, error)
}

func (s *Server) sessionClient() (sessionAPI, error) {
	if s.session != nil {
		return s.session, nil
	}
	if _, ok := core.Alive(); !ok {
		return nil, mihomoapi.ErrCoreNotRunning
	}
	return mihomoapi.Default()
}

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/v1/logs" {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	lines, err := core.SessionLog()
	if err != nil {
		http.Error(w, "log unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]any{"lines": lines})
}

func (s *Server) connections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/v1/connections" {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	client, err := s.sessionClient()
	if err != nil {
		writeProxyError(w, err)
		return
	}
	list, err := client.Connections(r.Context())
	if err != nil {
		writeProxyError(w, err)
		return
	}
	writeJSON(w, map[string]any{"connections": list})
}
