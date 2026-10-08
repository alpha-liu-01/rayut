package core

import "testing"

func TestOverlayControllerReplacesListenSecret(t *testing.T) {
	root := map[string]any{
		"secret":              "hidden",
		"external-controller": "0.0.0.0:9090",
		"external-ui":         "/tmp/ui",
		"log-file":            "/tmp/mihomo.log",
		"proxies":             []any{},
	}
	overlayController(root, "generated")
	if root["external-controller"] != ExternalController {
		t.Fatalf("controller %v", root["external-controller"])
	}
	if root["secret"] != "generated" {
		t.Fatal("secret was not replaced")
	}
	if _, ok := root["external-ui"]; ok {
		t.Fatal("external-ui remained")
	}
	if _, ok := root["log-file"]; ok {
		t.Fatal("log-file remained")
	}
	if root["unified-delay"] != true {
		t.Fatal("unified delay was not enabled")
	}
}
