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
