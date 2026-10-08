package core

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"gopkg.in/yaml.v3"
)

// ExternalController is the loopback API injected when mihomo starts.
// The GUI never dials it. rayutd does, with the secret kept in the runtime dir.
const ExternalController = "127.0.0.1:19090"

func prepareRunConfig() (string, error) {
	data, err := os.ReadFile(paths.Profile)
	if err != nil {
		return "", err
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", err
	}
	if root == nil {
		root = map[string]any{}
	}
	secret, err := externalSecret()
	if err != nil {
		return "", err
	}
	overlayController(root, secret)
	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(paths.Runtime, "run.yaml")
	if err := os.WriteFile(dest, out, 0o600); err != nil {
		return "", err
	}
	return dest, nil
}

func overlayController(root map[string]any, secret string) {
	root["external-controller"] = ExternalController
	root["secret"] = secret
	root["unified-delay"] = true
	delete(root, "external-controller-tls")
	delete(root, "external-controller-unix")
	delete(root, "external-controller-pipe")
	delete(root, "external-ui")
}

func externalSecret() (string, error) {
	path := filepath.Join(paths.Runtime, "controller.secret")
	if data, err := os.ReadFile(path); err == nil {
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	text := hex.EncodeToString(buf)
	if err := os.MkdirAll(paths.Runtime, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(text+"\n"), 0o600); err != nil {
		return "", err
	}
	return text, nil
}

// ExternalSecret is the bearer token for the injected mihomo controller.
func ExternalSecret() (string, error) {
	return externalSecret()
}
