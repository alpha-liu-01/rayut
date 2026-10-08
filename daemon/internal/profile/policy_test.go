package profile

import (
	"strings"
	"testing"
)

func TestPrepareRejectsUnsafeProfiles(t *testing.T) {
	cases := []struct {
		name    string
		content string
		code    string
	}{
		{name: "empty", content: " \n", code: "empty"},
		{name: "too large", content: strings.Repeat("a", MaxProfileBytes+1), code: "too large"},
		{name: "invalid", content: ":\n: :", code: "invalid yaml"},
		{name: "file", content: "proxies:\n  - {name: a, server: file:///tmp/x}\n", code: "file scheme"},
		{name: "hook", content: "hooks:\n  start: /bin/true\nproxies: []\n", code: "hook"},
		{name: "file provider", content: "proxy-providers:\n  a:\n    type: file\n    path: /tmp/a.yaml\n", code: "hook"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			_, err := prepare(item.content)
			if codeOf(err) != item.code {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestPrepareAllowsLoopbackAndAddsTun(t *testing.T) {
	doc, err := prepare("allow-lan: true\nexternal-controller: 0.0.0.0:9090\nsecret: hidden\nbind-address: 0.0.0.0\nproxies: []\n")
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc.YAML)
	if !strings.Contains(text, "stack: gvisor") || !strings.Contains(text, "auto-redir: false") {
		t.Fatalf("missing tun base: %s", text)
	}
	if strings.Contains(text, "0.0.0.0") || strings.Contains(text, "hidden") {
		t.Fatalf("control settings remained: %s", text)
	}
}

func TestPrepareWrapsProxyList(t *testing.T) {
	doc, err := prepare("- name: lab\n  type: socks5\n  server: 127.0.0.1\n  port: 1080\n")
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc.YAML)
	if !strings.Contains(text, "name: lab") || !strings.Contains(text, "MATCH,Rayut") || len(doc.Payload) != 0 {
		t.Fatalf("wrapped list: %s", text)
	}
}

func TestPrepareKeepsProxyGroups(t *testing.T) {
	raw := "proxies:\n- name: a\n  type: socks5\n  server: 127.0.0.1\n  port: 1\nproxy-groups:\n- name: 日本\n  type: select\n  proxies: [a]\n- name: 自动\n  type: url-test\n  proxies: [a]\nrules:\n- MATCH,日本\n"
	doc, err := prepare(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc.YAML)
	if !strings.Contains(text, "日本") || !strings.Contains(text, "自动") || strings.Contains(text, "name: Rayut") {
		t.Fatalf("groups were flattened: %s", text)
	}
	if !strings.Contains(text, "unified-delay: true") {
		t.Fatalf("unified delay missing: %s", text)
	}
}

func TestPrepareKeepsShareLinkPayload(t *testing.T) {
	link := "vless://00000000-0000-0000-0000-000000000000@127.0.0.1:443?encryption=none#lab\n"
	doc, err := prepare(link)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc.YAML), "subscription.payload") || string(doc.Payload) != strings.TrimSpace(link) {
		t.Fatalf("payload was not kept")
	}
}
