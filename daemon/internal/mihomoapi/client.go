package mihomoapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/core"
)

const delayTestURL = "https://www.gstatic.com/generate_204"

var (
	ErrNotFound       = errors.New("not found")
	ErrNotSelectable  = errors.New("not selectable")
	ErrRejected       = errors.New("selection rejected")
	ErrDelay          = errors.New("delay failed")
	ErrTimeout        = errors.New("timeout")
	ErrController     = errors.New("controller unavailable")
	ErrName           = errors.New("invalid name")
	ErrCoreNotRunning = errors.New("core not running")
)

type Group struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Now        string `json:"now"`
	Selectable bool   `json:"selectable"`
	Nodes      []Node `json:"nodes"`
}

type Node struct {
	Name  string `json:"name"`
	Delay int    `json:"delay"`
}

type Client struct {
	base   string
	secret string
	http   *http.Client
}

func New(base, secret string) *Client {
	return &Client{
		base:   strings.TrimRight(base, "/"),
		secret: secret,
		http: &http.Client{
			Timeout: 12 * time.Second,
			Transport: &http.Transport{
				Proxy: nil,
			},
		},
	}
}

func Default() (*Client, error) {
	secret, err := core.ExternalSecret()
	if err != nil || secret == "" {
		return nil, ErrController
	}
	return New("http://"+core.ExternalController, secret), nil
}

type proxyPayload struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Now     string   `json:"now"`
	All     []string `json:"all"`
	TestURL string   `json:"testUrl"`
	History []struct {
		Delay int `json:"delay"`
	} `json:"history"`
}

type proxyDocument struct {
	Proxies map[string]proxyPayload `json:"proxies"`
}

type providerDocument struct {
	Providers map[string]struct {
		Proxies []proxyPayload `json:"proxies"`
	} `json:"providers"`
}

func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	code, body, err := c.get(ctx, "/proxies", "")
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, ErrController
	}
	var doc proxyDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, ErrController
	}
	nodes := map[string]proxyPayload{}
	for name, item := range doc.Proxies {
		if item.Name == "" {
			item.Name = name
		}
		nodes[item.Name] = item
	}
	if code, body, err = c.get(ctx, "/providers/proxies", ""); err == nil && code == http.StatusOK {
		var providers providerDocument
		if json.Unmarshal(body, &providers) == nil {
			for _, item := range providers.Providers {
				for _, proxy := range item.Proxies {
					if proxy.Name == "" || !validName(proxy.Name) {
						continue
					}
					if _, ok := nodes[proxy.Name]; ok {
						continue
					}
					nodes[proxy.Name] = proxy
				}
			}
		}
	}
	groups := make([]Group, 0)
	for _, item := range doc.Proxies {
		if !isPolicyGroup(item) {
			continue
		}
		group := Group{
			Name:       item.Name,
			Type:       item.Type,
			Now:        item.Now,
			Selectable: selectable(item.Type),
			Nodes:      make([]Node, 0, len(item.All)),
		}
		for _, name := range item.All {
			if !validName(name) {
				continue
			}
			delay := 0
			if node, ok := nodes[name]; ok {
				delay = latestDelay(node)
			}
			group.Nodes = append(group.Nodes, Node{Name: name, Delay: delay})
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

func (c *Client) Select(ctx context.Context, group, name string) error {
	if !validName(group) || !validName(name) {
		return ErrName
	}
	payload, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return ErrRejected
	}
	path, raw := escapedPath("/proxies", group)
	code, body, err := c.send(ctx, http.MethodPut, path, raw, "", payload)
	if err != nil {
		return err
	}
	return selectionError(code, body)
}

func (c *Client) Delay(ctx context.Context, name string) (int, error) {
	if !validName(name) {
		return 0, ErrName
	}
	query := url.Values{"url": {c.delayURL(ctx, name)}, "timeout": {"5000"}}.Encode()
	path, raw := escapedPath("/proxies", name)
	path += "/delay"
	raw += "/delay"
	code, body, err := c.send(ctx, http.MethodGet, path, raw, query, nil)
	if err != nil {
		return 0, err
	}
	if code == http.StatusNotFound {
		provider, ok, err := c.providerOf(ctx, name)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, ErrNotFound
		}
		path, raw = escapedProviderProxy(provider, name)
		code, body, err = c.send(ctx, http.MethodGet, path, raw, query, nil)
		if err != nil {
			return 0, err
		}
	}
	return delayResult(code, body)
}

func (c *Client) delayURL(ctx context.Context, name string) string {
	code, body, err := c.get(ctx, "/proxies", "")
	if err != nil || code != http.StatusOK {
		return delayTestURL
	}
	var doc proxyDocument
	if json.Unmarshal(body, &doc) != nil {
		return delayTestURL
	}
	for _, item := range doc.Proxies {
		if !isPolicyGroup(item) || !validTestURL(item.TestURL) {
			continue
		}
		for _, member := range item.All {
			if member == name {
				return item.TestURL
			}
		}
	}
	return delayTestURL
}

func validTestURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := parsed.Hostname()
	if host == "" || strings.EqualFold(host, "localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return false
	}
	return true
}

func (c *Client) providerOf(ctx context.Context, name string) (string, bool, error) {
	code, body, err := c.get(ctx, "/providers/proxies", "")
	if err != nil {
		return "", false, err
	}
	if code != http.StatusOK {
		return "", false, nil
	}
	var providers providerDocument
	if err := json.Unmarshal(body, &providers); err != nil {
		return "", false, ErrController
	}
	for provider, item := range providers.Providers {
		if !validName(provider) {
			continue
		}
		for _, proxy := range item.Proxies {
			if proxy.Name == name {
				return provider, true, nil
			}
		}
	}
	return "", false, nil
}

func (c *Client) get(ctx context.Context, path, query string) (int, []byte, error) {
	return c.send(ctx, http.MethodGet, path, path, query, nil)
}

func (c *Client) send(ctx context.Context, method, path, rawPath, query string, payload []byte) (int, []byte, error) {
	host := strings.TrimPrefix(strings.TrimPrefix(c.base, "http://"), "https://")
	scheme := "http"
	if strings.HasPrefix(c.base, "https://") {
		scheme = "https"
	}
	target := url.URL{Scheme: scheme, Host: host, Path: path, RawPath: rawPath, RawQuery: query}
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return 0, nil, ErrController
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return 0, nil, ErrTimeout
		}
		return 0, nil, ErrController
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, ErrController
	}
	return resp.StatusCode, body, nil
}

func escapedPath(prefix, name string) (string, string) {
	return prefix + "/" + name, prefix + "/" + url.PathEscape(name)
}

func escapedProviderProxy(provider, name string) (string, string) {
	path := "/providers/proxies/" + provider + "/" + name + "/healthcheck"
	raw := "/providers/proxies/" + url.PathEscape(provider) + "/" + url.PathEscape(name) + "/healthcheck"
	return path, raw
}

func selectionError(code int, body []byte) error {
	switch code {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusBadRequest:
		if strings.Contains(string(body), "Must be a Selector") {
			return ErrNotSelectable
		}
		return ErrRejected
	default:
		return ErrController
	}
}

func delayResult(code int, body []byte) (int, error) {
	switch code {
	case http.StatusOK:
		var payload struct {
			Delay int `json:"delay"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || payload.Delay <= 0 {
			return 0, ErrDelay
		}
		return payload.Delay, nil
	case http.StatusNotFound:
		return 0, ErrNotFound
	case http.StatusGatewayTimeout:
		return 0, ErrTimeout
	default:
		return 0, ErrDelay
	}
}

func isPolicyGroup(item proxyPayload) bool {
	if item.Name == "" || item.Name == "GLOBAL" || len(item.All) == 0 {
		return false
	}
	switch item.Type {
	case "Selector", "URLTest", "Fallback", "LoadBalance", "Relay", "Compatible":
		return true
	default:
		return false
	}
}

func selectable(kind string) bool {
	switch kind {
	case "Selector", "URLTest", "Fallback":
		return true
	default:
		return false
	}
}

func latestDelay(item proxyPayload) int {
	if len(item.History) == 0 {
		return 0
	}
	delay := item.History[len(item.History)-1].Delay
	if delay < 0 {
		return 0
	}
	return delay
}

func validName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 512 {
		return false
	}
	return !strings.ContainsRune(name, 0) && !strings.ContainsAny(name, "\r\n")
}
