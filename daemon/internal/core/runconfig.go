package core

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/lan"
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"github.com/alpha-liu-01/rayut/daemon/internal/profile"
	"gopkg.in/yaml.v3"
)

// ExternalController is the loopback API injected when mihomo starts.
// The GUI never dials it. rayutd does, with the secret kept in the runtime dir.
const ExternalController = "127.0.0.1:19090"

// MixedPort is the inbound proxy used when the profile did not set one.
// The external controller never moves onto this port.
const MixedPort = 7890

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
	profile.EnsureIPv6(root)
	disableStoredSelection(root)
	pinSavedSelection(root)
	applyLAN(root, lan.Enabled())
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

// disableStoredSelection keeps the node written into the profile. mihomo
// otherwise reloads the previous selection from its cache on every start.
func disableStoredSelection(root map[string]any) {
	section, _ := root["profile"].(map[string]any)
	if section == nil {
		section = map[string]any{}
		root["profile"] = section
	}
	section["store-selected"] = false
}

// pinSavedSelection copies a manual group's saved now into the field mihomo
// actually reads. The now field is only for the interface; an empty
// default-selected makes the group fall through to its first member.
func pinSavedSelection(root map[string]any) {
	groups, _ := root["proxy-groups"].([]any)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := group["type"].(string)
		if !strings.EqualFold(strings.TrimSpace(kind), "select") {
			continue
		}
		now, _ := group["now"].(string)
		now = strings.TrimSpace(now)
		if now == "" {
			continue
		}
		group["default-selected"] = now
	}
}

// applyLAN changes only the inbound proxy reachability. The controller
// address is applied afterwards and stays on loopback either way.
func applyLAN(root map[string]any, on bool) {
	if on {
		root["allow-lan"] = true
		root["bind-address"] = "*"
	} else {
		root["allow-lan"] = false
		root["bind-address"] = "127.0.0.1"
	}
	if firstProxyPort(root) == 0 {
		root["mixed-port"] = MixedPort
	}
}

// ListenPort is the proxy port another device would try. It prefers the
// profile's own mixed port, then any other inbound port, then MixedPort.
func ListenPort() int {
	data, err := os.ReadFile(paths.Profile)
	if err != nil {
		return MixedPort
	}
	var root map[string]any
	if yaml.Unmarshal(data, &root) != nil {
		return MixedPort
	}
	if port := firstProxyPort(root); port > 0 {
		return port
	}
	return MixedPort
}

func firstProxyPort(root map[string]any) int {
	for _, key := range []string{"mixed-port", "port", "socks-port", "redir-port", "tproxy-port"} {
		if port := portValue(root[key]); port > 0 {
			return port
		}
	}
	return 0
}

func portValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case uint64:
		if typed > 1<<31 {
			return 0
		}
		return int(typed)
	case float64:
		return int(typed)
	case string:
		port, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return port
	default:
		return 0
	}
}

func overlayController(root map[string]any, secret string) {
	root["external-controller"] = ExternalController
	root["secret"] = secret
	root["unified-delay"] = true
	delete(root, "external-controller-tls")
	delete(root, "external-controller-unix")
	delete(root, "external-controller-pipe")
	delete(root, "external-ui")
	delete(root, "log-file")
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
