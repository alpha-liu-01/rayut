package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const editSample = `proxies:
  - name: Node
    type: vmess
    server: example.com
    port: 443
    uuid: super-secret-password
    alterId: 0
    cipher: auto
    network: ws
    tls: true
    udp: true
    servername: example.com
    skip-cert-verify: true
    client-fingerprint: chrome
    ws-opts:
      path: /ray
      headers:
        Host: example.com
proxy-groups:
  - name: 日本
    type: select
    proxies:
      - Node
rules:
  - MATCH,日本
`

func TestStructuredEditKeepsUncoveredFields(t *testing.T) {
	doc, err := ApplyProxyEdits(editSample, []ProxyEdit{{
		Index:   0,
		Name:    "Node",
		Type:    "vmess",
		Server:  "example.com",
		Port:    8443,
		Network: "ws",
		TLS:     true,
		UDP:     true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"8443", "/ray", "client-fingerprint:", "skip-cert-verify:", "servername:", "MATCH,日本", "super-secret-password", "alterId:", "cipher:"} {
		if !strings.Contains(doc.Text, needle) {
			t.Fatalf("text lost %q", needle)
		}
	}
	raw, err := json.Marshal(doc.Proxies)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret-password") || strings.Contains(string(raw), "client-fingerprint") {
		t.Fatal("structured fields echoed a secret or an uncovered field")
	}
	if len(doc.Proxies) != 1 || !doc.Proxies[0].HasSecret || doc.Proxies[0].Port != 8443 {
		t.Fatalf("projection = %+v", doc.Proxies)
	}
	again, err := ParseDocument(doc.Text)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(again.Text, "/ray") || again.Proxies[0].Port != 8443 {
		t.Fatal("switching views dropped a field")
	}
}

func TestRenameKeepsGroupReference(t *testing.T) {
	doc, err := ApplyProxyEdits(editSample, []ProxyEdit{{
		Index:   0,
		Name:    "NodeB",
		Type:    "vmess",
		Server:  "example.com",
		Port:    443,
		Network: "ws",
		TLS:     true,
		UDP:     true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(doc.Text), &parsed); err != nil {
		t.Fatal(err)
	}
	groups, _ := parsed["proxy-groups"].([]any)
	group, _ := groups[0].(map[string]any)
	list, _ := group["proxies"].([]any)
	if len(list) != 1 || scalarText(list[0]) != "NodeB" {
		t.Fatal("group still points at the old name")
	}
	if !strings.Contains(doc.Text, "/ray") {
		t.Fatal("rename dropped an uncovered field")
	}
}

func TestNewSecretIsWrittenButNotProjected(t *testing.T) {
	doc, err := ApplyProxyEdits(editSample, []ProxyEdit{{
		Index:   0,
		Name:    "Node",
		Type:    "vmess",
		Server:  "example.com",
		Port:    443,
		Network: "ws",
		TLS:     true,
		UDP:     true,
		Secret:  "replacement-secret-value",
	}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(doc.Proxies)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "replacement-secret-value") || strings.Contains(string(raw), "super-secret-password") {
		t.Fatal("structured fields echoed a secret")
	}
	if !strings.Contains(doc.Text, "replacement-secret-value") || strings.Contains(doc.Text, "super-secret-password") {
		t.Fatal("secret was not replaced in the text")
	}
	if !strings.Contains(doc.Text, "/ray") {
		t.Fatal("replacing a secret dropped an uncovered field")
	}
}

func TestBadFieldDoesNotEchoSecret(t *testing.T) {
	_, err := ApplyProxyEdits(editSample, []ProxyEdit{{
		Index:   0,
		Name:    "Node",
		Type:    "vmess",
		Server:  "example.com",
		Port:    0,
		Network: "ws",
		TLS:     true,
		UDP:     true,
		Secret:  "another-secret-value",
	}})
	if err == nil || err.Error() != "bad field" || strings.Contains(err.Error(), "another-secret-value") || strings.Contains(err.Error(), "super-secret-password") {
		t.Fatalf("error = %v", err)
	}
}

func TestRejectedEditKeepsActive(t *testing.T) {
	dir := t.TempDir()
	active := []byte(editSample)
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last-good.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error {
		return os.ErrInvalid
	}}
	view, err := store.EditCurrent(editSample)
	if err == nil || err.Error() != "invalid config" {
		t.Fatalf("edit error = %v", err)
	}
	if strings.Contains(err.Error(), "super-secret-password") || view.Error != "invalid config" {
		t.Fatalf("view error leaked or changed: %q", view.Error)
	}
	got, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil || string(got) != editSample {
		t.Fatal("active changed")
	}
	good, err := os.ReadFile(filepath.Join(dir, "last-good.yaml"))
	if err != nil || string(good) != editSample {
		t.Fatal("last good changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "candidate.yaml")); !os.IsNotExist(err) {
		t.Fatal("candidate should not remain")
	}
}

func TestEditPortActivatesWithoutDroppingFields(t *testing.T) {
	dir := t.TempDir()
	active := []byte(editSample)
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last-good.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	state := stateFile{CurrentName: "订阅", CurrentKind: "subscription", CurrentHost: "example.com", RuleTemplate: "lan"}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	edited, err := ApplyProxyEdits(editSample, []ProxyEdit{{
		Index: 0, Name: "Node", Type: "vmess", Server: "example.com", Port: 8443, Network: "ws", TLS: true, UDP: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(path string) error {
		tested, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(tested)
		if !strings.Contains(text, "8443") || !strings.Contains(text, "/ray") || !strings.Contains(text, "super-secret-password") || !strings.Contains(text, "client-fingerprint:") {
			return os.ErrInvalid
		}
		return nil
	}}
	view, err := store.EditCurrent(edited.Text)
	if err != nil {
		t.Fatal(err)
	}
	if view.Candidate.State != "validated" || view.Candidate.Kind != "subscription" || view.RuleTemplate != "lan" {
		t.Fatalf("candidate = %+v template %s", view.Candidate, view.RuleTemplate)
	}
	still, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil || string(still) != editSample {
		t.Fatal("edit replaced active before activate")
	}
	activated, err := store.Activate()
	if err != nil {
		t.Fatal(err)
	}
	if activated.RuleTemplate != "lan" {
		t.Fatalf("template = %s", activated.RuleTemplate)
	}
	next, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(next)
	for _, needle := range []string{"8443", "/ray", "client-fingerprint:", "super-secret-password", "MATCH,日本"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("activated file lost %q", needle)
		}
	}
	good, err := os.ReadFile(filepath.Join(dir, "last-good.yaml"))
	if err != nil || string(good) != text {
		t.Fatal("last good was not updated with the activated file")
	}
}

func TestWriteEditSample(t *testing.T) {
	dir := os.Getenv("RAYUT_EDIT_OUT")
	if dir == "" {
		t.Skip()
	}
	edited, err := ApplyProxyEdits(editSample, []ProxyEdit{{
		Index: 0, Name: "Node", Type: "vmess", Server: "example.com", Port: 8443, Network: "ws", TLS: true, UDP: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepare(edited.Text)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "edited.yaml"), prepared.YAML, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBrokenEditWithURLKeepsActive(t *testing.T) {
	dir := t.TempDir()
	active := []byte(editSample)
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last-good.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	store := &Store{Dir: dir, Test: func(string) error {
		called = true
		return nil
	}}
	_, err := store.EditCurrent(editSample + "https://example.com/sub\nBROKEN\n")
	if err == nil || err.Error() != "invalid yaml" || strings.Contains(err.Error(), "example.com") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("broken yaml was handed to the core")
	}
	got, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil || string(got) != editSample {
		t.Fatal("broken yaml replaced active")
	}
	good, err := os.ReadFile(filepath.Join(dir, "last-good.yaml"))
	if err != nil || string(good) != editSample {
		t.Fatal("broken yaml replaced last good")
	}
	if _, err := os.Stat(filepath.Join(dir, "candidate.yaml")); !os.IsNotExist(err) {
		t.Fatal("candidate should not remain")
	}
	if _, err := os.Stat(filepath.Join(dir, "subscription.payload")); !os.IsNotExist(err) {
		t.Fatal("broken text was stored as a payload")
	}
}

func TestBrokenTextIsRejected(t *testing.T) {
	dir := t.TempDir()
	active := []byte(editSample)
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last-good.yaml"), active, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error { return nil }}
	_, err := store.EditCurrent("proxies: [\n  password: super-secret-password\n")
	if err == nil || err.Error() != "invalid yaml" || strings.Contains(err.Error(), "super-secret-password") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil || string(got) != editSample {
		t.Fatal("broken yaml replaced active")
	}
}
