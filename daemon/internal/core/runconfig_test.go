package core

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDisableStoredSelection(t *testing.T) {
	root := map[string]any{
		"profile": map[string]any{"store-selected": true},
	}
	disableStoredSelection(root)
	out, err := yaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "store-selected: false") {
		t.Fatalf("stored selection still on: %s", out)
	}
}

func TestPinSavedSelection(t *testing.T) {
	root := map[string]any{
		"proxy-groups": []any{
			map[string]any{"name": "漏网", "type": "select", "now": "日本 2"},
			map[string]any{"name": "自动", "type": "url-test", "now": "美国 备用 1"},
		},
	}
	pinSavedSelection(root)
	out, err := yaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "default-selected: 日本 2") {
		t.Fatalf("saved node was not pinned: %s", text)
	}
	if strings.Contains(text, "default-selected: 美国 备用 1") {
		t.Fatalf("automatic group was pinned: %s", text)
	}
}

func TestLANStaysOffTheController(t *testing.T) {
	root := map[string]any{
		"allow-lan":           true,
		"bind-address":        "0.0.0.0",
		"external-controller": "0.0.0.0:9090",
		"mixed-port":          7893,
	}
	applyLAN(root, false)
	overlayController(root, "controller-token")
	out, err := yaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "allow-lan: false") || !strings.Contains(text, "bind-address: 127.0.0.1") {
		t.Fatalf("lan still open: %s", text)
	}
	if !strings.Contains(text, "external-controller: 127.0.0.1:19090") || strings.Contains(text, "0.0.0.0:9090") {
		t.Fatalf("controller left the loopback: %s", text)
	}
	if strings.Contains(text, "mixed-port: 7890") || !strings.Contains(text, "mixed-port: 7893") {
		t.Fatalf("existing proxy port changed: %s", text)
	}

	opened := map[string]any{"external-controller": "0.0.0.0:9090"}
	applyLAN(opened, true)
	overlayController(opened, "controller-token")
	out, err = yaml.Marshal(opened)
	if err != nil {
		t.Fatal(err)
	}
	text = string(out)
	if !strings.Contains(text, "allow-lan: true") || !strings.Contains(text, "bind-address:") {
		t.Fatalf("lan did not open: %s", text)
	}
	if strings.Contains(text, "bind-address: 127.0.0.1") || strings.Contains(text, "0.0.0.0") {
		t.Fatalf("lan bind stayed closed or the controller moved: %s", text)
	}
	if !strings.Contains(text, "external-controller: 127.0.0.1:19090") {
		t.Fatalf("opening lan exposed the controller: %s", text)
	}
	if !strings.Contains(text, "mixed-port: 7890") {
		t.Fatalf("missing proxy port: %s", text)
	}
}
