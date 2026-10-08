package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"github.com/alpha-liu-01/rayut/daemon/internal/profile"
	"github.com/alpha-liu-01/rayut/daemon/internal/route"
)

func (s *Server) profileStore() *profile.Store {
	return &profile.Store{
		Dir:      filepath.Dir(paths.Profile),
		CheckDir: filepath.Join(paths.Runtime, "check"),
		Test:     core.Test,
		TunUp: func() bool {
			present, err := route.TunPresent()
			return err == nil && present
		},
	}
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, s.profileStore().View())
}

func (s *Server) importContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.profileStore().ImportContent(body.Name, body.Content)
	writeProfile(w, view, err)
}

func (s *Server) importURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		URL string `json:"url"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.profileStore().ImportURL(body.URL)
	writeProfile(w, view, err)
}

func (s *Server) activateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.profileStore().Activate()
	writeProfile(w, view, err)
}

func (s *Server) applyRuleTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.profileStore().ApplyTemplate(body.ID)
	writeProfile(w, view, err)
}

func (s *Server) profileDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.profileStore().CurrentDocument()
	writeDocument(w, doc, err)
}

func (s *Server) previewProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Text  string              `json:"text"`
		Edits []profile.ProxyEdit `json:"edits"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.profileStore().PreviewDocument(body.Text, body.Edits)
	writeDocument(w, doc, err)
}

func (s *Server) editProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.profileStore().EditCurrent(body.Text)
	writeProfile(w, view, err)
}

func writeDocument(w http.ResponseWriter, doc profile.Document, err error) {
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	writeJSON(w, doc)
}

func (s *Server) refreshProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	view, err := s.profileStore().Refresh()
	writeProfile(w, view, err)
}

func decodeProfile(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, profile.MaxProfileBytes+8192)
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		code := "invalid yaml"
		if strings.Contains(err.Error(), "too large") {
			code = "too large"
		}
		http.Error(w, code, http.StatusBadRequest)
		return false
	}
	return true
}

func writeProfile(w http.ResponseWriter, view profile.View, err error) {
	if err != nil {
		status := http.StatusBadRequest
		if profile.Code(err) == "tun running" {
			status = http.StatusConflict
		}
		if profile.Code(err) == "core missing" {
			status = http.StatusInternalServerError
		}
		http.Error(w, profile.Code(err), status)
		return
	}
	writeJSON(w, view)
}
