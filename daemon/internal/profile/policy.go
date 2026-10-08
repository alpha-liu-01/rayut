package profile

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

const MaxProfileBytes = 5 << 20

type document struct {
	YAML    []byte
	Payload []byte
}

func prepare(content string) (document, error) {
	raw := []byte(strings.TrimSpace(content))
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if len(raw) == 0 {
		return document{}, errCode("empty")
	}
	if len(raw) > MaxProfileBytes {
		return document{}, errCode("too large")
	}
	decoded, ok := decodeBase64YAML(raw)
	if ok {
		raw = decoded
	}
	if len(raw) > MaxProfileBytes {
		return document{}, errCode("too large")
	}
	if strings.Contains(strings.ToLower(string(raw)), "file://") {
		return document{}, errCode("file scheme")
	}
	if doc, ok := configDocument(raw); ok {
		if err := inspect(doc); err != nil {
			return document{}, err
		}
		applyBase(doc)
		out, err := yaml.Marshal(doc)
		if err != nil {
			return document{}, errCode("invalid yaml")
		}
		return document{YAML: out}, nil
	}
	if proxies, ok := proxyList(raw); ok {
		doc := map[string]any{"proxies": proxies}
		applyBase(doc)
		out, err := yaml.Marshal(doc)
		if err != nil {
			return document{}, errCode("invalid yaml")
		}
		return document{YAML: out}, nil
	}
	if link, ok := oneShareLine(raw); ok {
		proxy, err := parseShareLink(link)
		if err != nil {
			return document{}, err
		}
		if proxy != nil {
			doc := map[string]any{"proxies": []any{proxy}}
			applyBase(doc)
			out, err := yaml.Marshal(doc)
			if err != nil {
				return document{}, errCode("invalid yaml")
			}
			return document{YAML: out}, nil
		}
	}
	if bytes.Contains(raw, []byte("://")) {
		doc := map[string]any{
			"proxy-providers": map[string]any{
				"subscription": map[string]any{
					"type": "file",
					"path": "subscription.payload",
				},
			},
			"proxy-groups": []any{
				map[string]any{
					"name": "Rayut",
					"type": "select",
					"use":  []any{"subscription"},
				},
			},
			"rules": []any{"MATCH,Rayut"},
		}
		applyBase(doc)
		out, err := yaml.Marshal(doc)
		if err != nil {
			return document{}, errCode("invalid yaml")
		}
		return document{YAML: out, Payload: raw}, nil
	}
	return document{}, errCode("invalid yaml")
}

func oneShareLine(raw []byte) (string, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" || strings.Contains(text, "\n") || !strings.Contains(text, "://") {
		return "", false
	}
	return text, true
}

func decodeBase64YAML(raw []byte) ([]byte, bool) {
	if bytes.Contains(raw, []byte(":")) && bytes.Contains(raw, []byte("\n")) {
		return nil, false
	}
	compact := bytes.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, raw)
	if len(compact) == 0 || len(compact)%4 == 1 {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(string(compact))
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(string(compact))
		if err != nil {
			return nil, false
		}
	}
	if !bytes.Contains(decoded, []byte(":")) {
		return nil, false
	}
	return decoded, true
}

func inspect(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			switch key {
			case "hooks", "command", "executable", "script", "external-controller-pipe", "external-controller-unix", "external-ui":
				return errCode("hook")
			case "type":
				if strings.EqualFold(strings.TrimSpace(fmt.Sprint(item)), "file") {
					return errCode("hook")
				}
			}
			if err := inspect(item); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range typed {
			if err := inspect(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func configDocument(raw []byte) (map[string]any, bool) {
	var doc map[string]any
	if yaml.Unmarshal(raw, &doc) != nil || len(doc) == 0 {
		return nil, false
	}
	if _, ok := doc["proxies"]; ok {
		return doc, true
	}
	if _, ok := doc["proxy-groups"]; ok {
		return doc, true
	}
	if _, ok := doc["rules"]; ok {
		return doc, true
	}
	if _, ok := doc["proxy-providers"]; ok {
		return doc, true
	}
	return nil, false
}

func proxyList(raw []byte) ([]any, bool) {
	var listed []map[string]any
	if yaml.Unmarshal(raw, &listed) == nil && len(listed) > 0 && isProxy(listed[0]) {
		items := make([]any, 0, len(listed))
		for i, item := range listed {
			if !isProxy(item) {
				return nil, false
			}
			if strings.TrimSpace(fmt.Sprint(item["name"])) == "" || fmt.Sprint(item["name"]) == "<nil>" {
				item["name"] = fmt.Sprintf("节点%d", i+1)
			}
			items = append(items, item)
		}
		return items, true
	}
	var one map[string]any
	if yaml.Unmarshal(raw, &one) == nil && isProxy(one) {
		if strings.TrimSpace(fmt.Sprint(one["name"])) == "" || fmt.Sprint(one["name"]) == "<nil>" {
			one["name"] = "节点1"
		}
		return []any{one}, true
	}
	return nil, false
}

func isProxy(item map[string]any) bool {
	_, hasType := item["type"]
	_, hasServer := item["server"]
	return hasType && hasServer
}

func loopbackEndpoint(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return true
	}
	host := value
	if parsedHost, _, err := net.SplitHostPort(value); err == nil {
		host = parsedHost
	}
	return loopbackHost(host)
}

func loopbackHost(value string) bool {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if value == "" || value == "<nil>" || strings.EqualFold(value, "localhost") {
		return true
	}
	ip := net.ParseIP(value)
	return ip != nil && ip.IsLoopback()
}

func applyBase(root map[string]any) {
	root["allow-lan"] = false
	root["bind-address"] = "127.0.0.1"
	root["find-process-mode"] = "off"
	root["unified-delay"] = true
	delete(root, "secret")
	if value, ok := root["external-controller"]; ok && !loopbackEndpoint(fmt.Sprint(value)) {
		delete(root, "external-controller")
	}
	ensureRouting(root)
	root["tun"] = map[string]any{
		"enable":                true,
		"stack":                 "gvisor",
		"auto-route":            true,
		"auto-redir":            false,
		"auto-detect-interface": true,
		"dns-hijack":            []any{"any:53"},
	}
	if _, ok := root["dns"]; !ok {
		root["dns"] = map[string]any{
			"enable":                  true,
			"listen":                  "127.0.0.1:1053",
			"ipv6":                    true,
			"enhanced-mode":           "fake-ip",
			"fake-ip-range":           "198.18.0.1/16",
			"nameserver":              []any{"1.1.1.1"},
			"proxy-server-nameserver": []any{"1.1.1.1"},
		}
	}
}

func ensureRouting(root map[string]any) {
	if groups, ok := root["proxy-groups"].([]any); ok && len(groups) > 0 {
		if _, ok := root["rules"]; ok {
			return
		}
		name := "Rayut"
		if first, ok := groups[0].(map[string]any); ok {
			if text := strings.TrimSpace(fmt.Sprint(first["name"])); text != "" && text != "<nil>" {
				name = text
			}
		}
		root["rules"] = []any{"MATCH," + name}
		return
	}
	proxies, ok := root["proxies"].([]any)
	if !ok || len(proxies) == 0 {
		return
	}
	names := make([]any, 0, len(proxies))
	for _, item := range proxies {
		proxy, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(proxy["name"]))
		if name == "" || name == "<nil>" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return
	}
	root["proxy-groups"] = []any{
		map[string]any{"name": "Rayut", "type": "select", "proxies": names},
	}
	root["rules"] = []any{"MATCH,Rayut"}
}

type codedError struct {
	code string
}

func (e *codedError) Error() string { return e.code }

func errCode(code string) error { return &codedError{code: code} }

func Code(err error) string { return codeOf(err) }

func codeOf(err error) string {
	if err == nil {
		return ""
	}
	if coded, ok := err.(*codedError); ok {
		return coded.code
	}
	return "invalid config"
}
