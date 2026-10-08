package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const templateSample = "proxies:\n- name: lab\n  type: socks5\n  server: example.com\n  port: 443\n  password: super-secret-password\nproxy-groups:\n- name: 日本\n  type: select\n  proxies: [lab]\nrules:\n- MATCH,日本\n"

func TestTemplatesKeepCredentialsAndTarget(t *testing.T) {
	for _, id := range []string{"global", "lan", "lan-china"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte(templateSample), 0o600); err != nil {
			t.Fatal(err)
		}
		store := &Store{Dir: dir, CheckDir: t.TempDir(), Test: func(string) error { return nil }}
		view, err := store.ApplyTemplate(id)
		if err != nil {
			t.Fatal(id, err)
		}
		if view.RuleTemplate != id {
			t.Fatalf("%s template = %s", id, view.RuleTemplate)
		}
		body, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "super-secret-password") || !strings.Contains(text, "example.com") || !strings.Contains(text, "name: 日本") {
			t.Fatalf("%s dropped profile fields", id)
		}
		if !strings.Contains(text, "MATCH,日本") {
			t.Fatalf("%s lost the existing proxy target", id)
		}
		if id == "global" && strings.Contains(text, "192.168.0.0/16") {
			t.Fatal("global template bypasses LAN")
		}
		if id != "global" && !strings.Contains(text, "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve") {
			t.Fatalf("%s does not bypass LAN", id)
		}
		if id == "lan-china" && !strings.Contains(text, "DOMAIN-SUFFIX,cn,DIRECT") {
			t.Fatal("china template missing common suffix")
		}
		if id == "lan" && strings.Contains(text, "DOMAIN-SUFFIX,cn,DIRECT") {
			t.Fatal("lan template includes china suffixes")
		}
	}
}

func TestRejectedTemplateKeepsRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte(templateSample), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, CheckDir: t.TempDir(), Test: func(string) error { return errCode("invalid config") }}
	if _, err := store.ApplyTemplate("lan"); err == nil || Code(err) != "invalid config" {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil || string(body) != templateSample {
		t.Fatal("rejected template overwrote the active rules")
	}
	if good, readErr := os.ReadFile(filepath.Join(dir, "last-good.yaml")); readErr == nil && string(good) != templateSample {
		t.Fatal("rejected template replaced the saved rules")
	}
	view := store.View()
	if view.RuleTemplate != "" {
		t.Fatal(view.RuleTemplate)
	}
}

func TestUnknownTemplateKeepsRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte(templateSample), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error { return nil }}
	if _, err := store.ApplyTemplate("broken"); err == nil || Code(err) != "unknown template" {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if string(body) != templateSample {
		t.Fatal("unknown template overwrote the active rules")
	}
}

func TestTemplateRefusesWhileTunUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte(templateSample), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error { return nil }, TunUp: func() bool { return true }}
	if _, err := store.ApplyTemplate("global"); err == nil || Code(err) != "tun running" {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if string(body) != templateSample {
		t.Fatal("template changed while tun was up")
	}
}

func TestActivateClearsTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte(templateSample), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, CheckDir: t.TempDir(), Test: func(string) error { return nil }}
	if _, err := store.ApplyTemplate("lan"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportContent("本地", "proxies:\n- name: lab\n  type: socks5\n  server: example.com\n  port: 1\n"); err != nil {
		t.Fatal(err)
	}
	view, err := store.Activate()
	if err != nil {
		t.Fatal(err)
	}
	if view.RuleTemplate != "" {
		t.Fatal(view.RuleTemplate)
	}
}

func TestWriteTemplateSamples(t *testing.T) {
	dir := os.Getenv("RAYUT_TEMPLATE_OUT")
	if dir == "" {
		t.Skip()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepare("proxies:\n- name: lab\n  type: socks5\n  server: example.com\n  port: 1\n  password: test-password\n")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if yaml.Unmarshal(prepared.YAML, &doc) != nil {
		t.Fatal("sample yaml")
	}
	for _, id := range []string{"global", "lan", "lan-china"} {
		rules, ok := templateRules(id, proxyPolicy(doc))
		if !ok {
			t.Fatal(id)
		}
		applied := make([]any, 0, len(rules))
		for _, rule := range rules {
			applied = append(applied, rule)
		}
		doc["rules"] = applied
		out, err := yaml.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".yaml"), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
