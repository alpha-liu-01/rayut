package profile

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/redact"
	"gopkg.in/yaml.v3"
)

func TestShareLinksBecomeProxies(t *testing.T) {
	vmess, err := json.Marshal(map[string]string{
		"v": "2", "ps": "lab", "add": "example.com", "port": "443",
		"id": "11111111-2222-3333-4444-555555555555", "aid": "0", "scy": "auto",
		"net": "tcp", "type": "none", "tls": "tls", "sni": "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	user := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:test-password"))
	cases := []struct {
		name string
		link string
		kind string
	}{
		{name: "ss", link: "ss://" + user + "@example.com:8388#lab", kind: "ss"},
		{name: "vmess", link: "vmess://" + base64.StdEncoding.EncodeToString(vmess) + "#ignored", kind: "vmess"},
		{name: "vless", link: "vless://11111111-2222-3333-4444-555555555555@example.com:443?encryption=none&security=tls&sni=example.com&type=tcp#lab", kind: "vless"},
		{name: "trojan", link: "trojan://test-password@example.com:443?security=tls&sni=example.com#lab", kind: "trojan"},
		{name: "hysteria2", link: "hysteria2://test-password@example.com:443?sni=example.com#lab", kind: "hysteria2"},
		{name: "hy2", link: "hy2://test-password@example.com:443?sni=example.com#lab", kind: "hysteria2"},
		{name: "tuic", link: "tuic://11111111-2222-3333-4444-555555555555:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&sni=example.com&alpn=h3#lab", kind: "tuic"},
	}
	if dir := os.Getenv("RAYUT_SHARE_OUT"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			doc, err := prepare(item.link)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Payload) != 0 {
				t.Fatal("raw link was stored")
			}
			var root map[string]any
			if yaml.Unmarshal(doc.YAML, &root) != nil {
				t.Fatal("yaml")
			}
			proxies, _ := root["proxies"].([]any)
			if len(proxies) != 1 {
				t.Fatalf("proxies %d", len(proxies))
			}
			proxy, _ := proxies[0].(map[string]any)
			if proxy["type"] != item.kind || proxy["server"] != "example.com" || proxy["name"] != "lab" {
				t.Fatalf("type %v server %v name %v", proxy["type"], proxy["server"], proxy["name"])
			}
			if !strings.Contains(string(doc.YAML), "MATCH,Rayut") {
				t.Fatal("missing route")
			}
			if dir := os.Getenv("RAYUT_SHARE_OUT"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, item.name+".yaml"), doc.YAML, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestBrokenLinkIsRejected(t *testing.T) {
	_, err := prepare("vless://not-a-link")
	if codeOf(err) != "invalid link" {
		t.Fatalf("got %v", err)
	}
	_, err = prepare("wireguard://example.com:1")
	if codeOf(err) != "unrecognized link" {
		t.Fatalf("got %v", err)
	}
}

func TestBrokenLinkDoesNotReplaceActive(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "active.yaml")
	if err := os.WriteFile(active, []byte("current: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error { return nil }}
	_, err := store.ImportContent("本地", "vless://not-a-link")
	if codeOf(err) != "invalid link" {
		t.Fatalf("got %v", err)
	}
	body, err := os.ReadFile(active)
	if err != nil || string(body) != "current: true\n" {
		t.Fatal("active profile changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "candidate.yaml")); !os.IsNotExist(err) {
		t.Fatal("candidate was written")
	}
}

func TestCredentialLinkStaysOutOfLogs(t *testing.T) {
	const secret = "super-secret-password"
	link := "ss://aes-256-gcm:" + secret + "@example.com"
	_, err := prepare(link)
	if codeOf(err) != "invalid link" {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error included the credential")
	}
	if strings.Contains(redact.Text(link), secret) {
		t.Fatal("log redaction kept the credential")
	}
}
