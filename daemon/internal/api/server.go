package api

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
	"github.com/alpha-liu-01/rayut/daemon/internal/killswitch"
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"github.com/alpha-liu-01/rayut/daemon/internal/route"
	"github.com/alpha-liu-01/rayut/daemon/internal/traffic"
)

const ListenAddr = "127.0.0.1:18771"

// HelperVersion and APIVersion are reported to the client. A mismatch is only
// a prompt to reconnect; the helper does not stop itself or the core.
const (
	HelperVersion = "0.1.25"
	APIVersion    = "1"
)

type Server struct {
	token   string
	mu      sync.Mutex
	coreMu  sync.Mutex
	http    *http.Server
	groups  groupAPI
	session sessionAPI
	traffic *traffic.Ledger
	delays  delayState
	watch   killswitch.Watch
}

type delayState struct {
	mu      sync.Mutex
	group   string
	running bool
	done    int
	total   int
}

func New() (*Server, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(buf)
	if err := os.MkdirAll(paths.Runtime, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(paths.Runtime+"/api.token", []byte(token+"\n"), 0o600); err != nil {
		return nil, err
	}
	if err := publishClientToken(token); err != nil {
		return nil, err
	}
	s := &Server{token: token, traffic: traffic.New(traffic.Path())}
	if owned, err := route.HasOwned(); err == nil && owned && killswitch.Enabled() {
		s.watch.Blocked = true
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", s.auth(s.health))
	mux.HandleFunc("/v1/status", s.auth(s.status))
	mux.HandleFunc("/v1/kill-switch", s.auth(s.killSwitch))
	mux.HandleFunc("/v1/tun/enable", s.auth(s.enable))
	mux.HandleFunc("/v1/tun/disable", s.auth(s.disable))
	mux.HandleFunc("/v1/profiles", s.auth(s.profiles))
	mux.HandleFunc("/v1/groups", s.auth(s.groupsRoot))
	mux.HandleFunc("/v1/groups/", s.auth(s.groupItem))
	mux.HandleFunc("/v1/profiles/import-content", s.auth(s.importContent))
	mux.HandleFunc("/v1/profiles/import-url", s.auth(s.importURL))
	mux.HandleFunc("/v1/profiles/activate", s.auth(s.activateProfile))
	mux.HandleFunc("/v1/profiles/refresh", s.auth(s.refreshProfile))
	mux.HandleFunc("/v1/profiles/document", s.auth(s.profileDocument))
	mux.HandleFunc("/v1/profiles/preview", s.auth(s.previewProfile))
	mux.HandleFunc("/v1/profiles/edit", s.auth(s.editProfile))
	mux.HandleFunc("/v1/rule-templates", s.auth(s.applyRuleTemplate))
	mux.HandleFunc("/v1/proxy-groups", s.auth(s.proxyGroups))
	mux.HandleFunc("/v1/proxy-groups/", s.auth(s.proxyGroupSelection))
	mux.HandleFunc("/v1/proxies/", s.auth(s.proxyDelay))
	mux.HandleFunc("/v1/logs", s.auth(s.logs))
	mux.HandleFunc("/v1/connections", s.auth(s.connections))
	mux.HandleFunc("/v1/traffic", s.auth(s.trafficView))
	s.http = &http.Server{
		Addr:              ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
}

func (s *Server) Serve(ln net.Listener) error {
	stop := make(chan struct{})
	go s.watchTraffic(stop)
	err := s.http.Serve(ln)
	close(stop)
	return err
}

func (s *Server) Shutdown() error {
	return s.http.Close()
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || (host != "127.0.0.1" && host != "::1") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		got := r.Header.Get("Authorization")
		want := "Bearer " + s.token
		if len(got) != len(want) || !hmac.Equal([]byte(got), []byte(want)) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.snapshot())
}

func (s *Server) enable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := core.Alive(); ok {
		s.coreMu.Lock()
		s.watch.WasUp = true
		s.watch.Blocked = false
		s.coreMu.Unlock()
		writeJSON(w, s.snapshot())
		return
	}
	if err := route.Recover(); err != nil {
		http.Error(w, "recover failed", http.StatusInternalServerError)
		return
	}
	if err := s.profileStore().SyncActive(); err != nil {
		http.Error(w, "start failed", http.StatusInternalServerError)
		return
	}
	if err := core.Start(); err != nil {
		if err.Error() == "profile missing" || err.Error() == "core missing" {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, "start failed", http.StatusInternalServerError)
		return
	}
	if err := core.WaitTun(15 * time.Second); err != nil {
		core.BeginStop()
		_ = core.Stop()
		_ = route.Recover()
		core.EndStop()
		s.coreMu.Lock()
		s.watch.WasUp = false
		s.watch.Blocked = false
		s.coreMu.Unlock()
		http.Error(w, "tun failed", http.StatusInternalServerError)
		return
	}
	if err := route.PinIPv6Gateways(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	s.coreMu.Lock()
	s.watch.WasUp = true
	s.watch.Blocked = false
	s.coreMu.Unlock()
	writeJSON(w, s.snapshot())
}

func (s *Server) disable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stopTun(); err != nil {
		http.Error(w, "disable failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.snapshot())
}

// StopTun is used when the session helper itself is asked to exit.
func (s *Server) StopTun() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopTun()
}

func (s *Server) stopTun() error {
	core.BeginStop()
	defer core.EndStop()
	s.sampleTraffic()
	if err := core.Stop(); err != nil {
		return err
	}
	s.sampleTraffic()
	if err := route.Recover(); err != nil {
		return err
	}
	s.coreMu.Lock()
	s.watch.WasUp = false
	s.watch.Blocked = false
	s.coreMu.Unlock()
	return nil
}

func (s *Server) snapshot() map[string]string {
	state := "stopped"
	if _, ok := core.Alive(); ok {
		state = "running"
	}
	tun := "absent"
	if present, err := route.TunPresent(); err == nil && present {
		tun = "present"
	}
	config := "ok"
	if _, err := os.Stat(paths.Profile); err != nil {
		config = "missing"
	}
	s.coreMu.Lock()
	blocked := s.watch.Blocked
	s.coreMu.Unlock()
	network := "open"
	if blocked {
		network = "blocked"
	}
	sw := "off"
	if killswitch.Enabled() {
		sw = "on"
	}
	return map[string]string{
		"mihomo":        state,
		"tun":           tun,
		"config":        config,
		"network":       network,
		"killSwitch":    sw,
		"helperVersion": HelperVersion,
		"apiVersion":    APIVersion,
		"coreVersion":   core.Version(),
	}
}

func (s *Server) killSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": killswitch.Enabled()})
	case http.MethodPost:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&body); err != nil {
			http.Error(w, "invalid", http.StatusBadRequest)
			return
		}
		if err := killswitch.Set(body.Enabled); err != nil {
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"enabled": killswitch.Enabled()})
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}

func (s *Server) noteCore() {
	_, alive := core.Alive()
	s.coreMu.Lock()
	shouldRecover := s.watch.Observe(alive, core.Stopping(), killswitch.Enabled())
	s.coreMu.Unlock()
	if shouldRecover {
		if err := route.Recover(); err != nil {
			fmt.Fprintln(os.Stderr, "recover")
		}
	}
}

func publishClientToken(token string) error {
	if err := os.WriteFile(paths.ClientToken, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	uid, gid := paths.Owner()
	if err := os.Chown(paths.ClientToken, uid, gid); err != nil {
		_ = os.Remove(paths.ClientToken)
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
