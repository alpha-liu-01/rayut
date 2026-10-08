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
