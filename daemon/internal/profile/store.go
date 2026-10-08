package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	Dir           string
	CheckDir      string
	Test          func(path string) error
	Client        *http.Client
	AllowLoopback bool
	TunUp         func() bool
}

type View struct {
	Current   Item   `json:"current"`
	Candidate Item   `json:"candidate"`
	LastGood  bool   `json:"lastGood"`
	Error     string `json:"error"`
}

type Item struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Host  string `json:"host"`
	State string `json:"state"`
}

type stateFile struct {
	CurrentName    string `json:"currentName"`
	CurrentKind    string `json:"currentKind"`
	CurrentHost    string `json:"currentHost"`
	CandidateName  string `json:"candidateName"`
	CandidateKind  string `json:"candidateKind"`
	CandidateHost  string `json:"candidateHost"`
	CandidateState string `json:"candidateState"`
	Error          string `json:"error"`
}

func (s *Store) View() View {
	_ = s.ensureLastGood()
	state := s.readState()
	currentState := "absent"
	if _, err := os.Stat(s.activePath()); err == nil {
		currentState = "current"
	}
	if state.CurrentName == "" && currentState == "current" {
		state.CurrentName = "当前"
		state.CurrentKind = "local"
	}
	candidateState := state.CandidateState
	if candidateState == "" {
		candidateState = "absent"
	}
	return View{
		Current: Item{
			Name:  state.CurrentName,
			Kind:  state.CurrentKind,
			Host:  state.CurrentHost,
			State: currentState,
		},
		Candidate: Item{
			Name:  state.CandidateName,
			Kind:  state.CandidateKind,
			Host:  state.CandidateHost,
			State: candidateState,
		},
		LastGood: fileExists(s.lastGoodPath()),
		Error:    state.Error,
	}
}

func (s *Store) ImportContent(name, content string) (View, error) {
	if err := s.ensureLastGood(); err != nil {
		return View{}, err
	}
	prepared, err := s.validate(content)
	if err != nil {
		_ = s.writeError(codeOf(err))
		return s.View(), err
	}
	meta := stateFile{
		CandidateName:  cleanName(name, "本地"),
		CandidateKind:  "local",
		CandidateState: "validated",
	}
	if err := s.commitCandidate(prepared, meta); err != nil {
		return View{}, err
	}
	return s.View(), nil
}

func (s *Store) ImportURL(raw string) (View, error) {
	if err := s.ensureLastGood(); err != nil {
		return View{}, err
	}
	body, host, err := s.download(raw)
	if err != nil {
		_ = s.writeError(codeOf(err))
		return s.View(), err
	}
	prepared, err := s.validate(string(body))
	if err != nil {
		_ = s.writeError(codeOf(err))
		return s.View(), err
	}
	if err := s.writeSecret(s.urlPath(), raw); err != nil {
		return View{}, err
	}
	meta := stateFile{
		CandidateName:  cleanName(host, "订阅"),
		CandidateKind:  "subscription",
		CandidateHost:  host,
		CandidateState: "validated",
	}
	if err := s.commitCandidate(prepared, meta); err != nil {
		return View{}, err
	}
	return s.View(), nil
}

func (s *Store) Refresh() (View, error) {
	urlText, err := os.ReadFile(s.urlPath())
	if err != nil || strings.TrimSpace(string(urlText)) == "" {
		coded := errCode("no subscription")
		_ = s.writeError(coded.Error())
		return s.View(), coded
	}
	return s.ImportURL(strings.TrimSpace(string(urlText)))
}

func (s *Store) Activate() (View, error) {
	if s.TunUp != nil && s.TunUp() {
		coded := errCode("tun running")
		return s.View(), coded
	}
	candidate, err := os.ReadFile(s.candidatePath())
	if err != nil || len(bytes.TrimSpace(candidate)) == 0 {
		coded := errCode("no candidate")
		return s.View(), coded
	}
	if err := s.installActive(candidate); err != nil {
		return View{}, err
	}
	state := s.readState()
	state.CurrentName = state.CandidateName
	state.CurrentKind = state.CandidateKind
	state.CurrentHost = state.CandidateHost
	if state.CurrentName == "" {
		state.CurrentName = "当前"
		state.CurrentKind = "local"
	}
	state.Error = ""
	if state.CurrentKind != "subscription" {
		_ = os.Remove(s.urlPath())
	}
	if err := s.writeState(state); err != nil {
		return View{}, err
	}
	return s.View(), nil
}

func (s *Store) validate(content string) ([]byte, error) {
	prepared, err := prepare(content)
	if err != nil {
		return nil, err
	}
	if len(prepared.Payload) > 0 {
		if err := s.atomic(filepath.Join(s.checkDir(), "subscription.payload"), prepared.Payload); err != nil {
			return nil, err
		}
		if err := s.atomic(s.payloadPath(), prepared.Payload); err != nil {
			return nil, err
		}
	}
	temp, err := os.CreateTemp(s.Dir, "check-*.yaml")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(prepared.YAML); err != nil {
		temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	if s.Test == nil {
		return nil, errCode("core missing")
	}
	if err := s.Test(tempName); err != nil {
		if err.Error() == "core missing" {
			return nil, errCode("core missing")
		}
		return nil, errCode("invalid config")
	}
	if len(prepared.Payload) == 0 {
		_ = os.Remove(s.payloadPath())
	}
	return prepared.YAML, nil
}

func (s *Store) checkDir() string {
	if s.CheckDir != "" {
		return s.CheckDir
	}
	return s.Dir
}

func (s *Store) commitCandidate(prepared []byte, meta stateFile) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	if err := s.atomic(s.candidatePath(), prepared); err != nil {
		return err
	}
	previous := s.readState()
	previous.CandidateName = meta.CandidateName
	previous.CandidateKind = meta.CandidateKind
	previous.CandidateHost = meta.CandidateHost
	previous.CandidateState = meta.CandidateState
	previous.Error = ""
	return s.writeState(previous)
}

func (s *Store) installActive(content []byte) error {
	if err := s.atomic(s.activePath(), content); err != nil {
		return err
	}
	return s.atomic(s.lastGoodPath(), content)
}

func (s *Store) ensureLastGood() error {
	if fileExists(s.lastGoodPath()) || !fileExists(s.activePath()) {
		return nil
	}
	body, err := os.ReadFile(s.activePath())
	if err != nil {
		return err
	}
	return s.atomic(s.lastGoodPath(), body)
}

func (s *Store) download(raw string) ([]byte, string, error) {
	parsed, err := validateURL(raw, s.AllowLoopback)
	if err != nil {
		return nil, "", err
	}
	client := s.Client
	if client == nil {
		client = SafeClient(20 * time.Second)
	}
	request, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", errCode("fetch failed")
	}
	request.Header.Set("User-Agent", subscriptionUserAgent)
	response, err := client.Do(request)
	if err != nil {
		var coded *codedError
		if errors.As(err, &coded) {
			return nil, "", coded
		}
		return nil, "", errCode("fetch failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", errCode("fetch failed")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxProfileBytes+1))
	if err != nil {
		return nil, "", errCode("fetch failed")
	}
	if len(body) > MaxProfileBytes {
		return nil, "", errCode("too large")
	}
	return body, parsed.Hostname(), nil
}

// subscriptionUserAgent asks the provider for a Clash profile. Without it,
// many subscriptions return a flat share-link list and the proxy groups disappear.
const subscriptionUserAgent = "clash-meta/1.19.32"

func SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: redirectPolicy(false),
	}
}

func redirectPolicy(allowLoopback bool) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errCode("redirect")
		}
		if _, err := validateURL(request.URL.String(), allowLoopback); err != nil {
			return err
		}
		return nil
	}
}

func validateURL(raw string, allowLoopback bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return nil, errCode("fetch failed")
	}
	if parsed.Scheme == "file" {
		return nil, errCode("file scheme")
	}
	if parsed.Scheme != "https" {
		return nil, errCode("fetch failed")
	}
	if parsed.User != nil {
		return nil, errCode("fetch failed")
	}
	host := parsed.Hostname()
	if !allowLoopback && forbiddenHost(host) {
		return nil, errCode("fetch failed")
	}
	return parsed, nil
}

func forbiddenHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast())
}

func (s *Store) writeError(code string) error {
	state := s.readState()
	state.Error = code
	state.CandidateName = ""
	state.CandidateKind = ""
	state.CandidateHost = ""
	state.CandidateState = "failed"
	_ = os.Remove(s.candidatePath())
	return s.writeState(state)
}

func (s *Store) readState() stateFile {
	body, err := os.ReadFile(s.statePath())
	if err != nil {
		return stateFile{}
	}
	var state stateFile
	if json.Unmarshal(body, &state) != nil {
		return stateFile{}
	}
	return state
}

func (s *Store) writeState(state stateFile) error {
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.writeSecret(s.statePath(), string(body))
}

func (s *Store) atomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if _, err := temp.Write(body); err != nil {
		temp.Close()
		os.Remove(tempName)
		return err
	}
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		os.Remove(tempName)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return err
	}
	return os.Rename(tempName, path)
}

func (s *Store) writeSecret(path, text string) error {
	return s.atomic(path, []byte(strings.TrimSpace(text)+"\n"))
}

func (s *Store) activePath() string    { return filepath.Join(s.Dir, "active.yaml") }
func (s *Store) lastGoodPath() string  { return filepath.Join(s.Dir, "last-good.yaml") }
func (s *Store) candidatePath() string { return filepath.Join(s.Dir, "candidate.yaml") }
func (s *Store) statePath() string     { return filepath.Join(s.Dir, "state.json") }
func (s *Store) urlPath() string       { return filepath.Join(s.Dir, "subscription.url") }
func (s *Store) payloadPath() string   { return filepath.Join(s.Dir, "subscription.payload") }

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func cleanName(name, fallback string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\n", " "))
	if name == "" {
		return fallback
	}
	runes := []rune(name)
	if len(runes) > 40 {
		return string(runes[:40])
	}
	return name
}
