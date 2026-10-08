package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/mihomoapi"
	"github.com/alpha-liu-01/rayut/daemon/internal/profile"
)

func (s *Server) groupsRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/v1/groups" {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	groups, err := s.profileStore().Groups()
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"groups": groups, "active": s.profileStore().ActiveGroup()})
}

func (s *Server) groupItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/groups/")
	switch rest {
	case "import-content":
		s.importGroupText(w, r)
	case "import-url":
		s.importGroupURL(w, r)
	default:
		s.groupAction(w, r, rest)
	}
}

func (s *Server) importGroupText(w http.ResponseWriter, r *http.Request) {
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
	groups, err := s.profileStore().ImportText(body.Name, body.Content)
	writeGroups(w, groups, err)
}

func (s *Server) importGroupURL(w http.ResponseWriter, r *http.Request) {
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
	groups, err := s.profileStore().ImportSubscription(body.URL)
	writeGroups(w, groups, err)
}

func (s *Server) groupAction(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if id == "" || strings.Contains(id, ".") {
		http.Error(w, "group missing", http.StatusNotFound)
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		s.groupDetail(w, id)
	case action == "" && r.Method == http.MethodPost:
		s.groupUpdate(w, r, id)
	case action == "nodes" && r.Method == http.MethodGet:
		s.groupNodes(w, id)
	case action == "selectors" && r.Method == http.MethodGet:
		s.groupSelectors(w, id)
	case action == "refresh" && r.Method == http.MethodPost:
		s.groupCall(w, id, func(store *profile.Store) ([]profile.GroupInfo, error) {
			return store.RefreshGroup(id)
		})
	case action == "use" && r.Method == http.MethodPost:
		s.groupCall(w, id, func(store *profile.Store) ([]profile.GroupInfo, error) {
			return store.UseGroup(id)
		})
	case action == "delete" && r.Method == http.MethodPost:
		s.groupCall(w, id, func(store *profile.Store) ([]profile.GroupInfo, error) {
			return store.DeleteGroup(id)
		})
	case action == "node-delete" && r.Method == http.MethodPost:
		s.groupDeleteNode(w, r, id)
	case action == "clear" && r.Method == http.MethodPost:
		s.groupNodesCall(w, id, func(store *profile.Store) ([]profile.NodeInfo, error) {
			return store.ClearNodes(id)
		})
	case action == "select" && r.Method == http.MethodPost:
		s.groupSelect(w, r, id)
	case action == "delay" && r.Method == http.MethodPost:
		s.groupDelay(w, r, id)
	case action == "delay-all" && r.Method == http.MethodPost:
		s.groupDelayAll(w, id)
	case action == "delay-all" && r.Method == http.MethodGet:
		s.delayProgress(w, id)
	case action == "document" && r.Method == http.MethodGet:
		s.groupDocument(w, id)
	case action == "edit" && r.Method == http.MethodPost:
		s.groupEdit(w, r, id)
	case action == "template" && r.Method == http.MethodPost:
		s.groupTemplate(w, r, id)
	case action == "export" && r.Method == http.MethodPost:
		s.groupExport(w, r, id)
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}

func (s *Server) groupCall(w http.ResponseWriter, id string, fn func(*profile.Store) ([]profile.GroupInfo, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups, err := fn(s.profileStore())
	writeGroups(w, groups, err)
}

func (s *Server) groupNodesCall(w http.ResponseWriter, id string, fn func(*profile.Store) ([]profile.NodeInfo, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes, err := fn(s.profileStore())
	writeNodes(w, nodes, err)
}

func (s *Server) groupDetail(w http.ResponseWriter, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store := s.profileStore()
	groups, err := store.Groups()
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	for _, group := range groups {
		if group.ID != id {
			continue
		}
		raw, err := store.GroupURL(id)
		if err != nil {
			http.Error(w, profile.Code(err), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"group": group, "url": raw})
		return
	}
	http.Error(w, "group missing", http.StatusNotFound)
}

func (s *Server) groupUpdate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	store := s.profileStore()
	if strings.TrimSpace(body.Name) != "" {
		if _, err := store.RenameGroup(id, body.Name); err != nil {
			http.Error(w, profile.Code(err), http.StatusBadRequest)
			return
		}
	}
	if strings.TrimSpace(body.URL) != "" {
		if err := store.SetGroupURL(id, body.URL); err != nil {
			http.Error(w, profile.Code(err), http.StatusBadRequest)
			return
		}
	}
	groups, err := store.Groups()
	writeGroups(w, groups, err)
}

func (s *Server) groupDeleteNode(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Index int `json:"index"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes, err := s.profileStore().DeleteNode(id, body.Index)
	writeNodes(w, nodes, err)
}

func (s *Server) groupNodes(w http.ResponseWriter, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes, err := s.profileStore().Nodes(id)
	writeNodes(w, nodes, err)
}

func (s *Server) groupSelectors(w http.ResponseWriter, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups, err := s.profileStore().Selectors(id)
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"groups": groups})
}

func (s *Server) groupSelect(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name  string `json:"name"`
		Group string `json:"group"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	store := s.profileStore()
	selector := strings.TrimSpace(body.Group)
	if store.ActiveGroup() == id {
		if _, ok := core.Alive(); ok {
			var err error
			if selector == "" {
				selector, err = store.SelectorName(id, body.Name)
			}
			if err != nil {
				s.mu.Unlock()
				http.Error(w, profile.Code(err), http.StatusBadRequest)
				return
			}
			views, err := store.Selectors(id)
			if err != nil {
				s.mu.Unlock()
				http.Error(w, profile.Code(err), http.StatusBadRequest)
				return
			}
			manual := false
			for _, view := range views {
				if view.Name == selector && view.Selectable {
					manual = true
				}
			}
			if !manual {
				s.mu.Unlock()
				http.Error(w, "not selectable", http.StatusBadRequest)
				return
			}
			s.mu.Unlock()
			client, err := mihomoapi.Default()
			if err != nil {
				http.Error(w, "controller unavailable", http.StatusConflict)
				return
			}
			if err := client.Select(r.Context(), selector, body.Name); err != nil {
				http.Error(w, "selection rejected", http.StatusConflict)
				return
			}
			s.mu.Lock()
			store = s.profileStore()
		}
	}
	nodes, err := store.SelectNode(id, strings.TrimSpace(body.Group), body.Name)
	s.mu.Unlock()
	writeNodes(w, nodes, err)
}

func (s *Server) groupDelay(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	store := s.profileStore()
	active := store.ActiveGroup() == id
	s.mu.Unlock()
	if !active {
		http.Error(w, "core not running", http.StatusConflict)
		return
	}
	if _, ok := core.Alive(); !ok {
		http.Error(w, "core not running", http.StatusConflict)
		return
	}
	client, err := mihomoapi.Default()
	if err != nil {
		http.Error(w, "controller unavailable", http.StatusConflict)
		return
	}
	delay, err := client.Delay(r.Context(), body.Name)
	if err != nil {
		http.Error(w, "delay failed", http.StatusConflict)
		return
	}
	s.mu.Lock()
	err = s.profileStore().SetDelay(id, body.Name, delay)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]int{"delay": delay})
}

func (s *Server) groupDelayAll(w http.ResponseWriter, id string) {
	s.mu.Lock()
	store := s.profileStore()
	if store.ActiveGroup() != id {
		s.mu.Unlock()
		http.Error(w, "core not running", http.StatusConflict)
		return
	}
	nodes, err := store.Nodes(id)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	if _, ok := core.Alive(); !ok {
		http.Error(w, "core not running", http.StatusConflict)
		return
	}
	s.delays.mu.Lock()
	if s.delays.running {
		done, total := s.delays.done, s.delays.total
		s.delays.mu.Unlock()
		writeJSON(w, map[string]any{"running": true, "done": done, "total": total})
		return
	}
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node.Name != "" {
			names = append(names, node.Name)
		}
	}
	s.delays.group = id
	s.delays.running = true
	s.delays.done = 0
	s.delays.total = len(names)
	s.delays.mu.Unlock()
	go s.runDelays(id, names)
	writeJSON(w, map[string]any{"running": true, "done": 0, "total": len(names)})
}

func (s *Server) runDelays(id string, names []string) {
	defer func() {
		s.delays.mu.Lock()
		s.delays.running = false
		s.delays.done = s.delays.total
		s.delays.mu.Unlock()
	}()
	client, err := mihomoapi.Default()
	if err != nil {
		return
	}
	for _, name := range names {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		delay, err := client.Delay(ctx, name)
		cancel()
		if err != nil {
			delay = -1
		}
		s.mu.Lock()
		_ = s.profileStore().SetDelay(id, name, delay)
		s.mu.Unlock()
		s.delays.mu.Lock()
		s.delays.done++
		s.delays.mu.Unlock()
	}
}

func (s *Server) delayProgress(w http.ResponseWriter, id string) {
	s.delays.mu.Lock()
	defer s.delays.mu.Unlock()
	if s.delays.group != id {
		writeJSON(w, map[string]any{"running": false, "done": 0, "total": 0})
		return
	}
	writeJSON(w, map[string]any{"running": s.delays.running, "done": s.delays.done, "total": s.delays.total})
}

func (s *Server) groupDocument(w http.ResponseWriter, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.profileStore().GroupDocument(id)
	writeDocument(w, doc, err)
}

func (s *Server) groupEdit(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Text string `json:"text"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.profileStore().EditGroup(id, body.Text)
	writeDocument(w, doc, err)
}

func (s *Server) groupTemplate(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	groups, err := s.profileStore().ApplyGroupTemplate(id, body.ID)
	writeGroups(w, groups, err)
}

func (s *Server) groupExport(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeProfile(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var text string
	var err error
	if strings.TrimSpace(body.Name) == "" {
		text, err = s.profileStore().ExportLinks(id)
	} else {
		text, err = s.profileStore().ExportLink(id, body.Name)
	}
	if err != nil {
		http.Error(w, profile.Code(err), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"text": text})
}

func writeGroups(w http.ResponseWriter, groups []profile.GroupInfo, err error) {
	if err != nil {
		http.Error(w, profile.Code(err), statusFor(err))
		return
	}
	writeJSON(w, map[string]any{"groups": groups})
}

func writeNodes(w http.ResponseWriter, nodes []profile.NodeInfo, err error) {
	if err != nil {
		http.Error(w, profile.Code(err), statusFor(err))
		return
	}
	writeJSON(w, map[string]any{"nodes": nodes})
}

func statusFor(err error) int {
	if profile.Code(err) == "tun running" || profile.Code(err) == "core not running" {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}
