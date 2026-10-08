// Share-link parsing is a Go translation of v2rayNG 6fe3893
// (https://github.com/2dust/v2rayNG), files under
// V2rayNG/app/src/main/java/com/v2ray/ang/fmt/:
// FmtBase.kt, ShadowsocksFmt.kt, VmessFmt.kt, VlessFmt.kt, TrojanFmt.kt,
// Hysteria2Fmt.kt.
// v2rayNG is Copyright (C) 2dust and contributors and is licensed under
// the GNU General Public License version 3. See THIRD_PARTY_NOTICES.
//
// That revision defines the tuic:// scheme but comments out the parser.
// The TUIC query names below follow FmtBase plus mihomo v1.19.32 TuicOption.

package profile

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

func parseShareLink(line string) (map[string]any, error) {
	line = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, " ", "%20"), "|", "%7C"))
	scheme, rest, ok := splitScheme(line)
	if !ok {
		return nil, nil
	}
	switch scheme {
	case "ss":
		return parseShadowsocks(rest)
	case "vmess":
		return parseVmess(line)
	case "vless":
		return parseVless(line)
	case "trojan":
		return parseTrojan(line)
	case "hysteria2", "hy2":
		return parseHysteria2(line)
	case "tuic":
		return parseTuic(line)
	default:
		return nil, errCode("unrecognized link")
	}
}

func splitScheme(line string) (string, string, bool) {
	index := strings.Index(line, "://")
	if index <= 0 {
		return "", "", false
	}
	scheme := strings.ToLower(line[:index])
	if strings.ContainsAny(scheme, " \t/#?") {
		return "", "", false
	}
	return scheme, line[index+3:], true
}

func parseShadowsocks(rest string) (map[string]any, error) {
	if proxy, err := parseShadowsocksSIP002("ss://" + rest); err == nil && proxy != nil {
		return proxy, nil
	}
	return parseShadowsocksLegacy(rest)
}

func parseShadowsocksSIP002(link string) (map[string]any, error) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.Hostname() == "" {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 || parsed.User == nil {
		return nil, errCode("invalid link")
	}
	method, password := shadowsocksUser(parsed.User)
	if method == "" || password == "" {
		return nil, errCode("invalid link")
	}
	proxy := baseProxy(linkName(parsed), "ss", parsed.Hostname(), port)
	proxy["cipher"] = method
	proxy["password"] = password
	proxy["udp"] = true
	return proxy, nil
}

func shadowsocksUser(info *url.Userinfo) (string, string) {
	user := info.Username()
	if password, ok := info.Password(); ok {
		return user, password
	}
	decoded, ok := decodeBase64(user)
	if !ok {
		return "", ""
	}
	method, password, found := strings.Cut(decoded, ":")
	if !found {
		return "", ""
	}
	return method, password
}

func parseShadowsocksLegacy(rest string) (map[string]any, error) {
	remarks := "none"
	if index := strings.LastIndex(rest, "#"); index >= 0 {
		remarks = decodeComponent(rest[index+1:])
		if remarks == "" {
			remarks = "none"
		}
		rest = rest[:index]
	}
	if index := strings.Index(rest, "@"); index > 0 {
		if decoded, ok := decodeBase64(rest[:index]); ok {
			rest = decoded + rest[index:]
		}
	} else if decoded, ok := decodeBase64(rest); ok {
		rest = decoded
	}
	method, after, ok := strings.Cut(rest, ":")
	if !ok {
		return nil, errCode("invalid link")
	}
	password, hostport, ok := strings.Cut(after, "@")
	if !ok {
		return nil, errCode("invalid link")
	}
	hostport = strings.TrimSuffix(hostport, "/")
	host, portText, err := splitHostPort(hostport)
	if err != nil {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || method == "" || password == "" || host == "" {
		return nil, errCode("invalid link")
	}
	proxy := baseProxy(remarks, "ss", host, port)
	proxy["cipher"] = strings.ToLower(method)
	proxy["password"] = password
	proxy["udp"] = true
	return proxy, nil
}

func parseVmess(link string) (map[string]any, error) {
	if strings.Contains(link, "?") && strings.Contains(link, "&") {
		return parseVmessStandard(link)
	}
	body := strings.TrimPrefix(link, "vmess://")
	if index := strings.Index(body, "#"); index >= 0 {
		body = body[:index]
	}
	decoded, ok := decodeBase64(body)
	if !ok {
		return nil, errCode("invalid link")
	}
	var code struct {
		PS       string `json:"ps"`
		Add      string `json:"add"`
		Port     string `json:"port"`
		ID       string `json:"id"`
		Aid      string `json:"aid"`
		SCY      string `json:"scy"`
		Net      string `json:"net"`
		Type     string `json:"type"`
		Host     string `json:"host"`
		Path     string `json:"path"`
		TLS      string `json:"tls"`
		SNI      string `json:"sni"`
		ALPN     string `json:"alpn"`
		FP       string `json:"fp"`
		Insecure string `json:"insecure"`
	}
	if json.Unmarshal([]byte(decoded), &code) != nil || code.Add == "" || code.Port == "" || code.ID == "" || code.Net == "" {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(code.Port)
	if err != nil || port <= 0 {
		return nil, errCode("invalid link")
	}
	name := code.PS
	if name == "" {
		name = "none"
	}
	proxy := baseProxy(name, "vmess", code.Add, port)
	proxy["uuid"] = code.ID
	proxy["alterId"] = atoiDefault(code.Aid, 0)
	cipher := code.SCY
	if cipher == "" {
		cipher = "auto"
	}
	proxy["cipher"] = cipher
	proxy["udp"] = true
	network := code.Net
	if network == "" {
		network = "tcp"
	}
	proxy["network"] = network
	applyTransport(proxy, queryMap{
		"type": network, "host": code.Host, "path": code.Path, "headerType": code.Type,
	})
	applyTLS(proxy, code.TLS, code.SNI, code.ALPN, code.FP, code.Insecure == "1", "", "")
	return proxy, nil
}

func parseVmessStandard(link string) (map[string]any, error) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.RawQuery == "" || parsed.Hostname() == "" {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 || parsed.User == nil {
		return nil, errCode("invalid link")
	}
	query := queryValues(parsed)
	proxy := baseProxy(linkName(parsed), "vmess", parsed.Hostname(), port)
	proxy["uuid"] = userText(parsed.User)
	proxy["alterId"] = 0
	proxy["cipher"] = "auto"
	proxy["udp"] = true
	applyTransport(proxy, query)
	applySecurity(proxy, query)
	return proxy, nil
}

func parseVless(link string) (map[string]any, error) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.RawQuery == "" || parsed.Hostname() == "" || parsed.User == nil {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 {
		return nil, errCode("invalid link")
	}
	query := queryValues(parsed)
	proxy := baseProxy(linkName(parsed), "vless", parsed.Hostname(), port)
	proxy["uuid"] = userText(parsed.User)
	proxy["udp"] = true
	if encryption := query["encryption"]; encryption != "" && encryption != "none" {
		proxy["encryption"] = encryption
	}
	if flow := query["flow"]; flow != "" {
		proxy["flow"] = flow
	}
	applyTransport(proxy, query)
	applySecurity(proxy, query)
	return proxy, nil
}

func parseTrojan(link string) (map[string]any, error) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.Hostname() == "" || parsed.User == nil {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 {
		return nil, errCode("invalid link")
	}
	proxy := baseProxy(linkName(parsed), "trojan", parsed.Hostname(), port)
	proxy["password"] = userText(parsed.User)
	proxy["udp"] = true
	if parsed.RawQuery == "" {
		proxy["network"] = "tcp"
		proxy["sni"] = parsed.Hostname()
		return proxy, nil
	}
	query := queryValues(parsed)
	applyTransport(proxy, query)
	security := query["security"]
	if security == "" {
		security = "tls"
	}
	applyTLS(proxy, security, query["sni"], query["alpn"], query["fp"], insecure(query), query["pbk"], query["sid"])
	return proxy, nil
}

func parseHysteria2(link string) (map[string]any, error) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.Hostname() == "" || parsed.User == nil {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 {
		return nil, errCode("invalid link")
	}
	query := queryValues(parsed)
	proxy := baseProxy(linkName(parsed), "hysteria2", parsed.Hostname(), port)
	proxy["password"] = userText(parsed.User)
	if sni := query["sni"]; sni != "" {
		proxy["sni"] = sni
	}
	if insecure(query) {
		proxy["skip-cert-verify"] = true
	}
	if fp := query["fp"]; fp != "" {
		proxy["fingerprint"] = fp
	}
	if alpn := splitCSV(query["alpn"]); len(alpn) > 0 {
		proxy["alpn"] = alpn
	}
	if password := query["obfs-password"]; password != "" {
		proxy["obfs"] = "salamander"
		proxy["obfs-password"] = password
	}
	if ports := query["mport"]; ports != "" {
		proxy["ports"] = ports
	}
	return proxy, nil
}

func parseTuic(link string) (map[string]any, error) {
	parsed, err := url.Parse(link)
	if err != nil || parsed.Hostname() == "" || parsed.User == nil {
		return nil, errCode("invalid link")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 {
		return nil, errCode("invalid link")
	}
	id, password, ok := strings.Cut(userText(parsed.User), ":")
	if !ok || id == "" || password == "" {
		return nil, errCode("invalid link")
	}
	query := queryValues(parsed)
	proxy := baseProxy(linkName(parsed), "tuic", parsed.Hostname(), port)
	proxy["uuid"] = id
	proxy["password"] = password
	if sni := query["sni"]; sni != "" {
		proxy["sni"] = sni
	}
	if insecure(query) {
		proxy["skip-cert-verify"] = true
	}
	if alpn := splitCSV(query["alpn"]); len(alpn) > 0 {
		proxy["alpn"] = alpn
	}
	if value := firstQuery(query, "congestion_control", "congestion-controller"); value != "" {
		proxy["congestion-controller"] = value
	}
	if value := firstQuery(query, "udp_relay_mode", "udp-relay-mode"); value != "" {
		proxy["udp-relay-mode"] = value
	}
	return proxy, nil
}

type queryMap map[string]string

func queryValues(parsed *url.URL) queryMap {
	values := parsed.Query()
	out := make(queryMap, len(values))
	for key, items := range values {
		if len(items) == 0 {
			continue
		}
		out[key] = items[0]
	}
	return out
}

func applyTransport(proxy map[string]any, query queryMap) {
	network := query["type"]
	if network == "" {
		network = "tcp"
	}
	if network == "h2" {
		network = "http"
	}
	proxy["network"] = network
	switch network {
	case "ws":
		opts := map[string]any{}
		if path := query["path"]; path != "" {
			opts["path"] = path
		}
		if host := query["host"]; host != "" {
			opts["headers"] = map[string]any{"Host": host}
		}
		if len(opts) > 0 {
			proxy["ws-opts"] = opts
		}
	case "grpc":
		if name := query["serviceName"]; name != "" {
			proxy["grpc-opts"] = map[string]any{"grpc-service-name": name}
		}
	}
}

func applySecurity(proxy map[string]any, query queryMap) {
	security := query["security"]
	if security != "tls" && security != "reality" {
		security = ""
	}
	applyTLS(proxy, security, query["sni"], query["alpn"], query["fp"], insecure(query), query["pbk"], query["sid"])
}

func applyTLS(proxy map[string]any, security, sni, alpn, fp string, skip bool, publicKey, shortID string) {
	if security == "tls" || security == "reality" {
		proxy["tls"] = true
	}
	if sni != "" {
		proxy["servername"] = sni
	}
	if fp != "" {
		proxy["client-fingerprint"] = fp
	}
	if items := splitCSV(alpn); len(items) > 0 {
		proxy["alpn"] = items
	}
	if skip {
		proxy["skip-cert-verify"] = true
	}
	if security == "reality" && publicKey != "" {
		reality := map[string]any{"public-key": publicKey}
		if shortID != "" {
			reality["short-id"] = shortID
		}
		proxy["reality-opts"] = reality
	}
}

func insecure(query queryMap) bool {
	for _, key := range []string{"insecure", "allowInsecure", "allow_insecure"} {
		if query[key] == "1" {
			return true
		}
	}
	return false
}

func firstQuery(query queryMap, keys ...string) string {
	for _, key := range keys {
		if query[key] != "" {
			return query[key]
		}
	}
	return ""
}

func splitCSV(value string) []any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func baseProxy(name, kind, server string, port int) map[string]any {
	if name == "" {
		name = "none"
	}
	return map[string]any{
		"name":   name,
		"type":   kind,
		"server": server,
		"port":   port,
	}
}

func linkName(parsed *url.URL) string {
	name := decodeComponent(parsed.EscapedFragment())
	if name == "" {
		name = "none"
	}
	return name
}

func userText(info *url.Userinfo) string {
	user := info.Username()
	password, ok := info.Password()
	if !ok {
		return user
	}
	return user + ":" + password
}

func decodeComponent(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func decodeBase64(value string) (string, bool) {
	value = strings.TrimSpace(value)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) > 0 {
			return string(decoded), true
		}
		trimmed := strings.TrimRight(value, "=")
		decoded, err = encoding.DecodeString(trimmed)
		if err == nil && len(decoded) > 0 {
			return string(decoded), true
		}
	}
	return "", false
}

func atoiDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitHostPort(value string) (string, string, error) {
	value = strings.TrimSuffix(value, "/")
	if strings.HasPrefix(value, "[") {
		host, port, found := strings.Cut(value, "]:")
		if !found {
			return "", "", errCode("invalid link")
		}
		return strings.TrimPrefix(host, "["), port, nil
	}
	host, port, found := strings.Cut(value, ":")
	if !found {
		return "", "", errCode("invalid link")
	}
	return host, port, nil
}
