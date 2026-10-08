package profile

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProxyField is one inline proxy as shown on the structured page.
// Secret values are not included.
type ProxyField struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Server    string `json:"server"`
	Port      int    `json:"port"`
	Network   string `json:"network"`
	TLS       bool   `json:"tls"`
	UDP       bool   `json:"udp"`
	HasSecret bool   `json:"hasSecret"`
}

// ProxyEdit patches one inline proxy. An empty Secret keeps the stored secret.
type ProxyEdit struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Server  string `json:"server"`
	Port    int    `json:"port"`
	Network string `json:"network"`
	TLS     bool   `json:"tls"`
	UDP     bool   `json:"udp"`
	Secret  string `json:"secret"`
}

// Document is the current profile text plus the fields the structured page can show.
type Document struct {
	Text    string       `json:"text"`
	Proxies []ProxyField `json:"proxies"`
}

// ParseDocument reads profile text and lists inline proxies without rewriting the text.
func ParseDocument(text string) (Document, error) {
	doc, err := loadDoc(text)
	if err != nil {
		return Document{}, err
	}
	return Document{Text: text, Proxies: projectProxies(doc)}, nil
}

// ApplyProxyEdits updates only the structured fields and remarshals the rest of the document.
func ApplyProxyEdits(text string, edits []ProxyEdit) (Document, error) {
	doc, err := loadDoc(text)
	if err != nil {
		return Document{}, err
	}
	if len(edits) == 0 {
		return Document{Text: text, Proxies: projectProxies(doc)}, nil
	}
	for _, edit := range edits {
		if err := applyProxyEdit(doc, edit); err != nil {
			return Document{}, err
		}
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return Document{}, errCode("invalid yaml")
	}
	return Document{Text: string(out), Proxies: projectProxies(doc)}, nil
}

func loadDoc(text string) (map[string]any, error) {
	if len(text) > MaxProfileBytes {
		return nil, errCode("too large")
	}
	raw := strings.TrimSpace(text)
	if raw == "" {
		return nil, errCode("empty")
	}
	var doc map[string]any
	if yaml.Unmarshal([]byte(raw), &doc) != nil || len(doc) == 0 {
		return nil, errCode("invalid yaml")
	}
	return doc, nil
}

func projectProxies(doc map[string]any) []ProxyField {
	proxies, ok := doc["proxies"].([]any)
	if !ok {
		return []ProxyField{}
	}
	fields := make([]ProxyField, 0, len(proxies))
	for index, item := range proxies {
		proxy, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fields = append(fields, ProxyField{
			Index:     index,
			Name:      scalarText(proxy["name"]),
			Type:      scalarText(proxy["type"]),
			Server:    scalarText(proxy["server"]),
			Port:      scalarPort(proxy["port"]),
			Network:   scalarText(proxy["network"]),
			TLS:       scalarTLS(proxy["tls"]),
			UDP:       scalarBool(proxy["udp"]),
			HasSecret: secretPresent(proxy),
		})
	}
	return fields
}

func applyProxyEdit(doc map[string]any, edit ProxyEdit) error {
	proxies, ok := doc["proxies"].([]any)
	if !ok || edit.Index < 0 || edit.Index >= len(proxies) {
		return errCode("bad field")
	}
	proxy, ok := proxies[edit.Index].(map[string]any)
	if !ok {
		return errCode("bad field")
	}
	name := strings.TrimSpace(edit.Name)
	kind := strings.TrimSpace(edit.Type)
	server := strings.TrimSpace(edit.Server)
	network := strings.TrimSpace(edit.Network)
	if name == "" || kind == "" || server == "" || edit.Port < 1 || edit.Port > 65535 {
		return errCode("bad field")
	}
	oldName := scalarText(proxy["name"])
	proxy["name"] = name
	proxy["type"] = kind
	proxy["server"] = server
	proxy["port"] = edit.Port
	if network == "" {
		delete(proxy, "network")
	} else {
		proxy["network"] = network
	}
	proxy["tls"] = edit.TLS
	proxy["udp"] = edit.UDP
	writeSecret(proxy, edit.Secret)
	if name != oldName {
		renameProxy(doc, oldName, name)
	}
	return nil
}

func writeSecret(proxy map[string]any, secret string) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return
	}
	switch strings.ToLower(scalarText(proxy["type"])) {
	case "vmess", "vless":
		proxy["uuid"] = secret
		return
	}
	if _, ok := proxy["uuid"]; ok {
		if _, hasPassword := proxy["password"]; !hasPassword {
			proxy["uuid"] = secret
			return
		}
	}
	proxy["password"] = secret
}

func renameProxy(doc map[string]any, oldName, newName string) {
	if oldName == "" || oldName == newName {
		return
	}
	if proxies, ok := doc["proxies"].([]any); ok {
		for _, item := range proxies {
			proxy, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if scalarText(proxy["dialer-proxy"]) == oldName {
				proxy["dialer-proxy"] = newName
			}
		}
	}
	groups, ok := doc["proxy-groups"].([]any)
	if !ok {
		return
	}
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		list, ok := group["proxies"].([]any)
		if !ok {
			continue
		}
		for index, name := range list {
			if scalarText(name) == oldName {
				list[index] = newName
			}
		}
	}
}

func secretPresent(proxy map[string]any) bool {
	for _, key := range []string{"password", "uuid", "token"} {
		value, ok := proxy[key]
		if !ok || value == nil {
			continue
		}
		text, isText := value.(string)
		if isText {
			if strings.TrimSpace(text) != "" {
				return true
			}
			continue
		}
		return true
	}
	return false
}

func scalarText(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func scalarPort(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case uint64:
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

func scalarBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func scalarTLS(value any) bool {
	if text, ok := value.(string); ok {
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "tls", "reality", "true", "1", "yes", "on":
			return true
		default:
			return false
		}
	}
	return scalarBool(value)
}
