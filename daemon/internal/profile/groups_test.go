package profile

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecondImportKeepsTheFirstGroup(t *testing.T) {
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }}
	if _, err := store.ImportText("甲", "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 1}\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportText("乙", "proxies:\n  - {name: beta, type: ss, server: example.com, port: 2}\n"); err != nil {
		t.Fatal(err)
	}
	groups, err := store.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups %+v", groups)
	}
	first, err := store.Nodes(groups[0].ID)
	if err != nil || len(first) != 1 || first[0].Name != "alpha" {
		t.Fatalf("first %+v %v", first, err)
	}
	if !groups[0].Active || groups[1].Active {
		t.Fatalf("active %+v", groups)
	}
}

func TestShareLinkAppendsToDefaultGroup(t *testing.T) {
	vmess, _ := json.Marshal(map[string]string{
		"v": "2", "ps": "lab", "add": "example.com", "port": "443",
		"id": "11111111-2222-3333-4444-555555555555", "aid": "0", "scy": "auto",
		"net": "tcp", "type": "none", "tls": "tls",
	})
	link := "vmess://" + base64.StdEncoding.EncodeToString(vmess)
	other, _ := json.Marshal(map[string]string{
		"v": "2", "ps": "lab-b", "add": "example.com", "port": "8443",
		"id": "11111111-2222-3333-4444-555555555555", "aid": "0", "scy": "auto",
		"net": "ws", "type": "none", "tls": "tls",
	})
	second := "vmess://" + base64.StdEncoding.EncodeToString(other)
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }}
	if _, err := store.ImportText("", link); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportText("", second); err != nil {
		t.Fatal(err)
	}
	groups, err := store.Groups()
	if err != nil || len(groups) != 1 || groups[0].Count != 2 || groups[0].Name != "默认" {
		t.Fatalf("groups %+v %v", groups, err)
	}
	nodes, err := store.Nodes(groups[0].ID)
	if err != nil || len(nodes) != 2 || !nodes[0].Shareable || !nodes[1].Shareable {
		t.Fatalf("nodes %+v %v", nodes, err)
	}
	raw, _ := json.Marshal(nodes)
	for _, secret := range []string{"11111111-2222-3333-4444-555555555555", "vmess://", "example.com"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("node list contains %s", secret)
		}
	}
	exported, err := store.ExportLinks(groups[0].ID)
	if err != nil || !strings.Contains(exported, "vmess://") {
		t.Fatalf("export %q %v", exported, err)
	}
	one, err := store.ExportLink(groups[0].ID, nodes[0].Name)
	if err != nil || !strings.HasPrefix(one, "vmess://") {
		t.Fatal(err)
	}
	if _, err := store.ExportLink(groups[0].ID, "missing"); err == nil || Code(err) != "not found" {
		t.Fatal(err)
	}
}

func TestBrokenImportKeepsGroups(t *testing.T) {
	store := &Store{Dir: t.TempDir(), Test: func(path string) error {
		body, _ := os.ReadFile(path)
		if strings.Contains(string(body), "broken-node") {
			return errCode("invalid config")
		}
		return nil
	}}
	groups, err := store.ImportText("甲", "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 1}\n")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(store.Dir, "groups", groups[0].ID, "config.yaml"))
	if _, err := store.ImportText("乙", "proxies:\n  - {name: broken-node, type: ss, server: example.com, port: 1}\n"); err == nil {
		t.Fatal("expected rejection")
	}
	after, _ := os.ReadFile(filepath.Join(store.Dir, "groups", groups[0].ID, "config.yaml"))
	if string(after) != string(before) {
		t.Fatal("rejected import changed the first group")
	}
	active, _ := os.ReadFile(filepath.Join(store.Dir, "active.yaml"))
	if string(active) != string(before) {
		t.Fatal("rejected import changed the active file")
	}
}

func TestUseGroupRefusesWhileTunIsUp(t *testing.T) {
	up := false
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }, TunUp: func() bool { return up }}
	if _, err := store.ImportText("甲", "proxies:\n  - {name: alpha, type: ss, server: example.com, port: 1}\n"); err != nil {
		t.Fatal(err)
	}
	groups, err := store.ImportText("乙", "proxies:\n  - {name: beta, type: ss, server: example.com, port: 2}\n")
	if err != nil {
		t.Fatal(err)
	}
	up = true
	if _, err := store.UseGroup(groups[1].ID); codeOf(err) != "tun running" {
		t.Fatalf("err %v", err)
	}
	got, _ := store.Groups()
	if !got[0].Active || got[1].Active {
		t.Fatalf("active changed %+v", got)
	}
	up = false
	if _, err := store.UseGroup(groups[1].ID); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(store.Dir, "active.yaml"))
	if !strings.Contains(string(body), "beta") || strings.Contains(string(body), "alpha") {
		t.Fatalf("active file %s", body)
	}
}

func TestMigrateKeepsExistingProfile(t *testing.T) {
	dir := t.TempDir()
	const marker = "name: kept-node"
	if err := os.WriteFile(filepath.Join(dir, "active.yaml"), []byte("proxies:\n  - {name: kept-node, type: ss, server: example.com, port: 1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"currentName":"已有","currentKind":"subscription"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: dir, Test: func(string) error { return nil }}
	groups, err := store.Groups()
	if err != nil || len(groups) != 1 || groups[0].Name != "已有" || groups[0].Kind != "subscription" || !groups[0].Active {
		t.Fatalf("%+v %v", groups, err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "groups", groups[0].ID, "config.yaml"))
	if !strings.Contains(string(body), marker) {
		t.Fatalf("migrated %s", body)
	}
	active, _ := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if !strings.Contains(string(active), marker) {
		t.Fatal("migration rewrote the original file")
	}
}

func TestSelectNodeUsesTheManualGroup(t *testing.T) {
	const doc = `proxies:
  - {name: alpha, type: ss, server: example.com, port: 1, cipher: aes-128-gcm, password: secret-alpha}
  - {name: beta, type: ss, server: example.com, port: 2, cipher: aes-128-gcm, password: secret-beta}
proxy-groups:
  - {name: 手动, type: select, proxies: [alpha, beta], now: alpha}
  - {name: 自动, type: url-test, proxies: [alpha, beta], url: https://www.gstatic.com/generate_204, interval: 300, now: beta}
rules:
  - MATCH,手动
`
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }}
	groups, err := store.ImportText("订阅", doc)
	if err != nil || len(groups) != 1 {
		t.Fatal(err)
	}
	selectors, err := store.Selectors(groups[0].ID)
	if err != nil || len(selectors) != 2 || !selectors[0].Selectable || selectors[1].Selectable {
		t.Fatalf("%+v %v", selectors, err)
	}
	raw, _ := json.Marshal(selectors)
	if strings.Contains(string(raw), "secret-alpha") || strings.Contains(string(raw), "example.com") {
		t.Fatal("selector list contains a secret or server")
	}
	if _, err := store.SelectNode(groups[0].ID, "自动", "alpha"); err != nil {
		t.Fatal(err)
	}
	selectors, err = store.Selectors(groups[0].ID)
	if err != nil || selectors[0].Now != "alpha" || selectors[1].Now != "beta" {
		t.Fatalf("url-test tap %+v %v", selectors, err)
	}
	if _, err := store.SelectNode(groups[0].ID, "手动", "beta"); err != nil {
		t.Fatal(err)
	}
	selectors, err = store.Selectors(groups[0].ID)
	if err != nil || selectors[0].Now != "beta" || selectors[1].Now != "beta" {
		t.Fatalf("%+v %v", selectors, err)
	}
}

func TestSelectNodeFollowsTheMatchRule(t *testing.T) {
	const doc = `proxies:
  - {name: alpha, type: ss, server: example.com, port: 1, cipher: aes-128-gcm, password: secret-alpha}
  - {name: japan, type: ss, server: example.com, port: 2, cipher: aes-128-gcm, password: secret-beta}
proxy-groups:
  - {name: 日本, type: select, proxies: [japan], now: japan}
  - {name: 漏网, type: select, proxies: [alpha, japan], now: alpha}
rules:
  - MATCH,漏网
`
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }}
	groups, err := store.ImportText("订阅", doc)
	if err != nil || len(groups) != 1 {
		t.Fatal(err)
	}
	if _, err := store.SelectNode(groups[0].ID, "日本", "japan"); err != nil {
		t.Fatal(err)
	}
	selectors, err := store.Selectors(groups[0].ID)
	if err != nil || selectors[0].Now != "japan" || selectors[1].Now != "japan" {
		t.Fatalf("%+v %v", selectors, err)
	}
	if !selectors[1].Nodes[1].Selected || selectors[1].Nodes[0].Selected || !selectors[0].Nodes[0].Selected {
		t.Fatalf("mark %+v", selectors)
	}
}

func TestSelectNodePointsMatchAtTheGroup(t *testing.T) {
	const doc = `proxies:
  - {name: alpha, type: ss, server: example.com, port: 1, cipher: aes-128-gcm, password: secret-alpha}
  - {name: japan, type: ss, server: example.com, port: 2, cipher: aes-128-gcm, password: secret-beta}
proxy-groups:
  - {name: 日本, type: select, proxies: [japan], now: japan}
  - {name: 漏网, type: select, proxies: [alpha, 日本], now: alpha}
rules:
  - MATCH,漏网
`
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }}
	groups, err := store.ImportText("订阅", doc)
	if err != nil || len(groups) != 1 {
		t.Fatal(err)
	}
	targets, err := store.RuntimeSelects(groups[0].ID, "日本", "japan")
	if err != nil || len(targets) != 2 || targets[0] != [2]string{"漏网", "日本"} || targets[1] != [2]string{"日本", "japan"} {
		t.Fatalf("%+v %v", targets, err)
	}
	if _, err := store.SelectNode(groups[0].ID, "日本", "japan"); err != nil {
		t.Fatal(err)
	}
	selectors, err := store.Selectors(groups[0].ID)
	if err != nil || selectors[0].Now != "japan" || selectors[1].Now != "日本" {
		t.Fatalf("%+v %v", selectors, err)
	}
	if !selectors[0].Nodes[0].Selected || selectors[1].Nodes[0].Selected || !selectors[1].Nodes[1].Selected {
		t.Fatalf("mark %+v", selectors)
	}
}

func TestSelectNodeUpdatesEveryManualGroup(t *testing.T) {
	const doc = `proxies:
  - {name: alpha, type: ss, server: example.com, port: 1, cipher: aes-128-gcm, password: secret-alpha}
  - {name: japan, type: ss, server: example.com, port: 2, cipher: aes-128-gcm, password: secret-beta}
proxy-groups:
  - {name: 节点, type: select, proxies: [alpha, japan], now: alpha}
  - {name: 媒体, type: select, proxies: [alpha, japan], now: alpha}
  - {name: 漏网, type: select, proxies: [alpha, japan], now: alpha}
  - {name: 最快, type: url-test, proxies: [alpha, japan], url: https://www.gstatic.com/generate_204, interval: 300}
rules:
  - MATCH,漏网
`
	store := &Store{Dir: t.TempDir(), Test: func(string) error { return nil }}
	groups, err := store.ImportText("订阅", doc)
	if err != nil || len(groups) != 1 {
		t.Fatal(err)
	}
	targets, err := store.RuntimeSelects(groups[0].ID, "最快", "japan")
	if err != nil || len(targets) != 3 {
		t.Fatalf("%+v %v", targets, err)
	}
	if _, err := store.SelectNode(groups[0].ID, "最快", "japan"); err != nil {
		t.Fatal(err)
	}
	selectors, err := store.Selectors(groups[0].ID)
	if err != nil || selectors[0].Now != "japan" || selectors[1].Now != "japan" || selectors[2].Now != "japan" {
		t.Fatalf("%+v %v", selectors, err)
	}
}
