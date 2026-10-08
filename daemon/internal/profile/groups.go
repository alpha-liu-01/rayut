package profile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultBucket = "default"

// GroupInfo is one stored profile group. It has no subscription URL.
type GroupInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Count    int    `json:"count"`
	Active   bool   `json:"active"`
	Template string `json:"template"`
}

// NodeInfo is one inline proxy on the home card. Secrets and the server
// address are not included.
type NodeInfo struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Network   string `json:"network"`
	Delay     int    `json:"delay"`
	Selected  bool   `json:"selected"`
	Shareable bool   `json:"shareable"`
}

// SelectorInfo is one Clash proxy-group inside a stored config group.
// Selectable is true only for type select. url-test and the other automatic
// kinds stay visible, but a tap cannot change them.
type SelectorInfo struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Selectable bool       `json:"selectable"`
	Traffic    bool       `json:"traffic"`
	Now        string     `json:"now"`
	Nodes      []NodeInfo `json:"nodes"`
}

type groupMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Bucket   string `json:"bucket,omitempty"`
	Template string `json:"template,omitempty"`
}

type groupIndex struct {
	Active string      `json:"active"`
	Groups []groupMeta `json:"groups"`
}

func (s *Store) Groups() ([]GroupInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	idx := s.readIndex()
	out := make([]GroupInfo, 0, len(idx.Groups))
	for _, meta := range idx.Groups {
		doc, _ := s.readGroupDoc(meta.ID)
		out = append(out, GroupInfo{
			ID:       meta.ID,
			Name:     meta.Name,
			Kind:     meta.Kind,
			Count:    len(projectProxies(doc)),
			Active:   meta.ID == idx.Active,
			Template: meta.Template,
		})
	}
	return out, nil
}

func (s *Store) Nodes(id string) ([]NodeInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	if _, ok := s.metaByID(id); !ok {
		return nil, errCode("group missing")
	}
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return nil, err
	}
	selected := effectiveNode(doc)
	links := s.readMap(s.groupLinks(id))
	delays := s.readDelayMap(id)
	fields := projectProxies(doc)
	out := make([]NodeInfo, 0, len(fields))
	for _, field := range fields {
		out = append(out, NodeInfo{
			Index:     field.Index,
			Name:      field.Name,
			Type:      field.Type,
			Network:   field.Network,
			Delay:     delays[field.Name],
			Selected:  field.Name != "" && field.Name == selected,
			Shareable: strings.TrimSpace(links[field.Name]) != "",
		})
	}
	return out, nil
}

// ImportText appends one share link to the default group, or stores a Clash
// document as its own group. Another group's files are left in place.
func (s *Store) ImportText(name, content string) ([]GroupInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(content)
	if link, ok := oneShareLine([]byte(raw)); ok {
		proxy, err := parseShareLink(link)
		if err != nil {
			return nil, err
		}
		if proxy != nil {
			if err := s.appendShare(proxy, link); err != nil {
				return nil, err
			}
			return s.Groups()
		}
	}
	prepared, err := s.validate(raw)
	if err != nil {
		return nil, err
	}
	label := cleanName(name, "本地")
	if _, err := s.addGroup(label, "manual", "", prepared, nil); err != nil {
		return nil, err
	}
	return s.Groups()
}

// ImportSubscription creates a group for a new URL, or refreshes the group
// that already stores that URL.
func (s *Store) ImportSubscription(raw string) ([]GroupInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	body, host, err := s.download(raw)
	if err != nil {
		return nil, err
	}
	prepared, err := s.validate(string(body))
	if err != nil {
		return nil, err
	}
	if id, ok := s.groupByURL(raw); ok {
		if err := s.replaceGroup(id, prepared); err != nil {
			return nil, err
		}
		return s.Groups()
	}
	if _, err := s.addGroup(cleanName(host, "订阅"), "subscription", strings.TrimSpace(raw), prepared, nil); err != nil {
		return nil, err
	}
	return s.Groups()
}

func (s *Store) RefreshGroup(id string) ([]GroupInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	meta, ok := s.metaByID(id)
	if !ok {
		return nil, errCode("group missing")
	}
	raw, err := os.ReadFile(s.groupURL(id))
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		return nil, errCode("no subscription")
	}
	body, _, err := s.download(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, err
	}
	prepared, err := s.validate(string(body))
	if err != nil {
		return nil, err
	}
	if err := s.replaceGroup(meta.ID, prepared); err != nil {
		return nil, err
	}
	return s.Groups()
}

// UseGroup makes one group the file mihomo will read. The proxy must be off.
func (s *Store) UseGroup(id string) ([]GroupInfo, error) {
	if s.TunUp != nil && s.TunUp() {
		return nil, errCode("tun running")
	}
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	if _, ok := s.metaByID(id); !ok {
		return nil, errCode("group missing")
	}
	idx := s.readIndex()
	idx.Active = id
	if err := s.writeIndex(idx); err != nil {
		return nil, err
	}
	if err := s.syncActive(); err != nil {
		return nil, err
	}
	return s.Groups()
}

func (s *Store) DeleteGroup(id string) ([]GroupInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	idx := s.readIndex()
	if id == idx.Active && s.TunUp != nil && s.TunUp() {
		return nil, errCode("tun running")
	}
	next := idx
	next.Groups = nil
	found := false
	for _, meta := range idx.Groups {
		if meta.ID == id {
			found = true
			continue
		}
		next.Groups = append(next.Groups, meta)
	}
	if !found {
		return nil, errCode("group missing")
	}
	if id == idx.Active {
		next.Active = ""
		if len(next.Groups) > 0 {
			next.Active = next.Groups[0].ID
		}
	}
	if err := os.RemoveAll(s.groupDir(id)); err != nil {
		return nil, err
	}
	if err := s.writeIndex(next); err != nil {
		return nil, err
	}
	if next.Active == "" {
		_ = os.Remove(s.activePath())
	} else if s.TunUp == nil || !s.TunUp() {
		if err := s.syncActive(); err != nil {
			return nil, err
		}
	}
	return s.Groups()
}

func (s *Store) DeleteNode(id string, index int) ([]NodeInfo, error) {
	return s.mutateNodes(id, func(doc map[string]any) error {
		proxies, _ := doc["proxies"].([]any)
		if index < 0 || index >= len(proxies) {
			return errCode("bad field")
		}
		proxy, _ := proxies[index].(map[string]any)
		name := ""
		if proxy != nil {
			name = scalarText(proxy["name"])
		}
		doc["proxies"] = append(proxies[:index], proxies[index+1:]...)
		dropProxyName(doc, name)
		if name != "" {
			links := s.readMap(s.groupLinks(id))
			delete(links, name)
			_ = s.writeMap(s.groupLinks(id), links)
			delays := s.readDelayMap(id)
			delete(delays, name)
			_ = s.writeDelayMap(id, delays)
		}
		return nil
	})
}

func (s *Store) ClearNodes(id string) ([]NodeInfo, error) {
	return s.mutateNodes(id, func(doc map[string]any) error {
		proxies, _ := doc["proxies"].([]any)
		for _, item := range proxies {
			proxy, _ := item.(map[string]any)
			if proxy != nil {
				dropProxyName(doc, scalarText(proxy["name"]))
			}
		}
		doc["proxies"] = []any{}
		_ = os.Remove(s.groupLinks(id))
		_ = os.Remove(s.groupDelays(id))
		return nil
	})
}

// Selectors lists the Clash proxy-groups stored in one config group.
// The core does not have to be running. Node rows omit secrets and servers.
func (s *Store) Selectors(id string) ([]SelectorInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	if _, ok := s.metaByID(id); !ok {
		return nil, errCode("group missing")
	}
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return nil, err
	}
	return projectSelectors(doc, s.readMap(s.groupLinks(id)), s.readDelayMap(id)), nil
}

// RuntimeSelects lists every manual group a tap has to update on the live core.
// Rules can send a site to any of these groups, so one tap moves all of them.
func (s *Store) RuntimeSelects(id, selector, name string) ([][2]string, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	if _, ok := s.metaByID(id); !ok {
		return nil, errCode("group missing")
	}
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return nil, err
	}
	return choiceTargets(doc, strings.TrimSpace(selector), strings.TrimSpace(name)), nil
}

// SelectNode writes now on one manual select group. An empty selector picks
// the first select group that lists the node. Automatic groups are refused.
func (s *Store) SelectNode(id, selector, name string) ([]NodeInfo, error) {
	name = strings.TrimSpace(name)
	selector = strings.TrimSpace(selector)
	if name == "" {
		return nil, errCode("invalid name")
	}
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	if _, ok := s.metaByID(id); !ok {
		return nil, errCode("group missing")
	}
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return nil, err
	}
	if err := applyChoice(doc, selector, name); err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, errCode("invalid yaml")
	}
	if err := s.atomic(s.groupConfig(id), out); err != nil {
		return nil, err
	}
	if s.readIndex().Active == id && (s.TunUp == nil || !s.TunUp()) {
		if err := s.syncActive(); err != nil {
			return nil, err
		}
	}
	return s.Nodes(id)
}

func (s *Store) SetDelay(id, name string, delay int) error {
	if _, ok := s.metaByID(id); !ok {
		return errCode("group missing")
	}
	delays := s.readDelayMap(id)
	delays[name] = delay
	return s.writeDelayMap(id, delays)
}

func (s *Store) GroupDocument(id string) (Document, error) {
	if err := s.ensureGroups(); err != nil {
		return Document{}, err
	}
	if _, ok := s.metaByID(id); !ok {
		return Document{}, errCode("group missing")
	}
	body, err := os.ReadFile(s.groupConfig(id))
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		return Document{}, errCode("profile missing")
	}
	return ParseDocument(string(body))
}

func (s *Store) EditGroup(id, content string) (Document, error) {
	if err := s.rejectLiveEdit(id); err != nil {
		return Document{}, err
	}
	prepared, err := s.validateDocument(content)
	if err != nil {
		return Document{}, err
	}
	if err := s.replaceGroup(id, prepared); err != nil {
		return Document{}, err
	}
	return s.GroupDocument(id)
}

func (s *Store) ApplyGroupTemplate(id, templateID string) ([]GroupInfo, error) {
	if err := s.rejectLiveEdit(id); err != nil {
		return nil, err
	}
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return nil, err
	}
	rules, ok := templateRules(templateID, proxyPolicy(doc))
	if !ok {
		return nil, errCode("unknown template")
	}
	applied := make([]any, 0, len(rules))
	for _, rule := range rules {
		applied = append(applied, rule)
	}
	doc["rules"] = applied
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, errCode("invalid yaml")
	}
	if _, err := s.validateDocument(string(out)); err != nil {
		return nil, err
	}
	if err := s.replaceGroup(id, out); err != nil {
		return nil, err
	}
	idx := s.readIndex()
	for i := range idx.Groups {
		if idx.Groups[i].ID == id {
			idx.Groups[i].Template = templateID
		}
	}
	if err := s.writeIndex(idx); err != nil {
		return nil, err
	}
	if id == idx.Active {
		state := s.readState()
		state.RuleTemplate = templateID
		_ = s.writeState(state)
	}
	return s.Groups()
}

func (s *Store) GroupURL(id string) (string, error) {
	if _, ok := s.metaByID(id); !ok {
		return "", errCode("group missing")
	}
	body, err := os.ReadFile(s.groupURL(id))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(body)), nil
}

func (s *Store) SetGroupURL(id, raw string) error {
	meta, ok := s.metaByID(id)
	if !ok {
		return errCode("group missing")
	}
	if meta.Kind != "subscription" {
		return errCode("no subscription")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errCode("empty")
	}
	if _, err := validateURL(raw, s.AllowLoopback); err != nil {
		return err
	}
	return s.writeSecret(s.groupURL(id), raw)
}

func (s *Store) RenameGroup(id, name string) ([]GroupInfo, error) {
	if err := s.ensureGroups(); err != nil {
		return nil, err
	}
	idx := s.readIndex()
	found := false
	for i := range idx.Groups {
		if idx.Groups[i].ID != id {
			continue
		}
		idx.Groups[i].Name = cleanName(name, idx.Groups[i].Name)
		found = true
	}
	if !found {
		return nil, errCode("group missing")
	}
	if err := s.writeIndex(idx); err != nil {
		return nil, err
	}
	return s.Groups()
}

// ExportLink returns one stored share line. Callers must not write it to a log.
func (s *Store) ExportLink(id, name string) (string, error) {
	if _, ok := s.metaByID(id); !ok {
		return "", errCode("group missing")
	}
	line := strings.TrimSpace(s.readMap(s.groupLinks(id))[name])
	if line == "" {
		return "", errCode("not found")
	}
	return line, nil
}

// ExportLinks returns the original share lines stored for this group.
// Clash-only nodes are omitted. Callers must not write the result to a log.
func (s *Store) ExportLinks(id string) (string, error) {
	if _, ok := s.metaByID(id); !ok {
		return "", errCode("group missing")
	}
	links := s.readMap(s.groupLinks(id))
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return "", err
	}
	var lines []string
	for _, field := range projectProxies(doc) {
		line := strings.TrimSpace(links[field.Name])
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}

// SelectorName is the Clash proxy-group that lists this node.
func (s *Store) SelectorName(id, node string) (string, error) {
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return "", err
	}
	groups, _ := doc["proxy-groups"].([]any)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok || !manualSelect(scalarText(group["type"])) {
			continue
		}
		if proxyListed(group, node) {
			name := scalarText(group["name"])
			if name == "" {
				return "", errCode("not found")
			}
			return name, nil
		}
	}
	return "", errCode("not found")
}

// ActiveGroup is the id currently installed for mihomo. It is empty when
// nothing has been stored.
func (s *Store) ActiveGroup() string {
	_ = s.ensureGroups()
	return s.readIndex().Active
}

// SyncActive copies the active group onto active.yaml. Enable calls it
// before starting the core, so a refresh done while the proxy was open is
// what the next start reads.
func (s *Store) SyncActive() error {
	if err := s.ensureGroups(); err != nil {
		return err
	}
	return s.syncActive()
}

func (s *Store) appendShare(proxy map[string]any, link string) error {
	idx := s.readIndex()
	id := ""
	for _, meta := range idx.Groups {
		if meta.Bucket == defaultBucket {
			id = meta.ID
			break
		}
	}
	if id == "" {
		name := scalarText(proxy["name"])
		if name == "" {
			name = "默认"
		}
		prepared, err := s.proxyDocument(map[string]any{"proxies": []any{proxy}})
		if err != nil {
			return err
		}
		created, err := s.addGroup("默认", "manual", "", prepared, map[string]string{name: link})
		if err != nil {
			return err
		}
		idx = s.readIndex()
		for i := range idx.Groups {
			if idx.Groups[i].ID == created {
				idx.Groups[i].Bucket = defaultBucket
			}
		}
		return s.writeIndex(idx)
	}
	existing, err := s.readGroupDoc(id)
	if err != nil {
		existing = map[string]any{}
	}
	name := uniqueProxyName(existing, scalarText(proxy["name"]))
	proxy["name"] = name
	addProxy(existing, proxy)
	out, err := yaml.Marshal(existing)
	if err != nil {
		return errCode("invalid yaml")
	}
	checked, err := s.validateDocument(string(out))
	if err != nil {
		return err
	}
	before, _ := os.ReadFile(s.groupConfig(id))
	if err := s.replaceGroup(id, checked); err != nil {
		if len(before) > 0 {
			_ = s.atomic(s.groupConfig(id), before)
		}
		return err
	}
	links := s.readMap(s.groupLinks(id))
	links[name] = link
	return s.writeMap(s.groupLinks(id), links)
}

func (s *Store) proxyDocument(doc map[string]any) ([]byte, error) {
	applyBase(doc)
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, errCode("invalid yaml")
	}
	return s.validateDocument(string(out))
}

func (s *Store) addGroup(name, kind, rawURL string, config []byte, links map[string]string) (string, error) {
	id := newGroupID()
	if err := os.MkdirAll(s.groupDir(id), 0o755); err != nil {
		return "", err
	}
	if err := s.atomic(s.groupConfig(id), config); err != nil {
		return "", err
	}
	if strings.TrimSpace(rawURL) != "" {
		if err := s.writeSecret(s.groupURL(id), rawURL); err != nil {
			return "", err
		}
	}
	if len(links) > 0 {
		if err := s.writeMap(s.groupLinks(id), links); err != nil {
			return "", err
		}
	}
	idx := s.readIndex()
	idx.Groups = append(idx.Groups, groupMeta{ID: id, Name: cleanName(name, "本地"), Kind: kind})
	if idx.Active == "" && (s.TunUp == nil || !s.TunUp()) {
		idx.Active = id
	}
	if err := s.writeIndex(idx); err != nil {
		return "", err
	}
	if idx.Active == id {
		if err := s.syncActive(); err != nil {
			return "", err
		}
	}
	return id, nil
}

func (s *Store) replaceGroup(id string, config []byte) error {
	if _, ok := s.metaByID(id); !ok {
		return errCode("group missing")
	}
	if err := s.atomic(s.groupConfig(id), config); err != nil {
		return err
	}
	if s.readIndex().Active == id && (s.TunUp == nil || !s.TunUp()) {
		return s.syncActive()
	}
	return nil
}

func (s *Store) mutateNodes(id string, change func(map[string]any) error) ([]NodeInfo, error) {
	if err := s.rejectLiveEdit(id); err != nil {
		return nil, err
	}
	doc, err := s.readGroupDoc(id)
	if err != nil {
		return nil, err
	}
	if err := change(doc); err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, errCode("invalid yaml")
	}
	if err := s.replaceGroup(id, out); err != nil {
		return nil, err
	}
	return s.Nodes(id)
}

func (s *Store) rejectLiveEdit(id string) error {
	if err := s.ensureGroups(); err != nil {
		return err
	}
	if _, ok := s.metaByID(id); !ok {
		return errCode("group missing")
	}
	if s.readIndex().Active == id && s.TunUp != nil && s.TunUp() {
		return errCode("tun running")
	}
	return nil
}

func (s *Store) ensureGroups() error {
	if fileExists(s.indexPath()) {
		return nil
	}
	idx := groupIndex{}
	if !fileExists(s.activePath()) {
		return s.writeIndex(idx)
	}
	body, err := os.ReadFile(s.activePath())
	if err != nil {
		return err
	}
	state := s.readState()
	kind := state.CurrentKind
	if kind != "subscription" {
		kind = "manual"
	}
	name := cleanName(state.CurrentName, "当前")
	id := newGroupID()
	if err := os.MkdirAll(s.groupDir(id), 0o755); err != nil {
		return err
	}
	if err := s.atomic(s.groupConfig(id), body); err != nil {
		return err
	}
	if urlBody, err := os.ReadFile(s.urlPath()); err == nil && strings.TrimSpace(string(urlBody)) != "" {
		if err := s.atomic(s.groupURL(id), urlBody); err != nil {
			return err
		}
	}
	idx.Active = id
	idx.Groups = []groupMeta{{
		ID:       id,
		Name:     name,
		Kind:     kind,
		Template: state.RuleTemplate,
	}}
	return s.writeIndex(idx)
}

func (s *Store) syncActive() error {
	idx := s.readIndex()
	if idx.Active == "" {
		return nil
	}
	body, err := os.ReadFile(s.groupConfig(idx.Active))
	if err != nil {
		return err
	}
	if err := s.installActive(body); err != nil {
		return err
	}
	urlBody, err := os.ReadFile(s.groupURL(idx.Active))
	if err == nil && len(strings.TrimSpace(string(urlBody))) > 0 {
		if err := s.atomic(s.urlPath(), urlBody); err != nil {
			return err
		}
	} else {
		_ = os.Remove(s.urlPath())
	}
	return nil
}

func (s *Store) groupByURL(raw string) (string, bool) {
	want := strings.TrimSpace(raw)
	for _, meta := range s.readIndex().Groups {
		body, err := os.ReadFile(s.groupURL(meta.ID))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(body)) == want {
			return meta.ID, true
		}
	}
	return "", false
}

func (s *Store) metaByID(id string) (groupMeta, bool) {
	for _, meta := range s.readIndex().Groups {
		if meta.ID == id {
			return meta, true
		}
	}
	return groupMeta{}, false
}

func (s *Store) readGroupDoc(id string) (map[string]any, error) {
	body, err := os.ReadFile(s.groupConfig(id))
	if err != nil {
		return nil, errCode("profile missing")
	}
	return loadDoc(string(body))
}

func (s *Store) readIndex() groupIndex {
	body, err := os.ReadFile(s.indexPath())
	if err != nil {
		return groupIndex{}
	}
	var idx groupIndex
	if json.Unmarshal(body, &idx) != nil {
		return groupIndex{}
	}
	return idx
}

func (s *Store) writeIndex(idx groupIndex) error {
	body, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return s.atomic(s.indexPath(), body)
}

func (s *Store) readMap(path string) map[string]string {
	body, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var out map[string]string
	if json.Unmarshal(body, &out) != nil || out == nil {
		return map[string]string{}
	}
	return out
}

func (s *Store) writeMap(path string, value map[string]string) error {
	if value == nil {
		value = map[string]string{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.atomic(path, body)
}

func (s *Store) readDelayMap(id string) map[string]int {
	body, err := os.ReadFile(s.groupDelays(id))
	if err != nil {
		return map[string]int{}
	}
	var out map[string]int
	if json.Unmarshal(body, &out) != nil || out == nil {
		return map[string]int{}
	}
	return out
}

func (s *Store) writeDelayMap(id string, value map[string]int) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.atomic(s.groupDelays(id), body)
}

func addProxy(doc map[string]any, proxy map[string]any) {
	proxies, _ := doc["proxies"].([]any)
	doc["proxies"] = append(proxies, proxy)
	name := scalarText(proxy["name"])
	groups, _ := doc["proxy-groups"].([]any)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok || !selectableKind(scalarText(group["type"])) {
			continue
		}
		list, _ := group["proxies"].([]any)
		group["proxies"] = append(list, name)
		if scalarText(group["now"]) == "" {
			group["now"] = name
		}
		return
	}
	doc["proxy-groups"] = append(groups, map[string]any{
		"name":    "Rayut",
		"type":    "select",
		"proxies": []any{name},
		"now":     name,
	})
	if _, ok := doc["rules"]; !ok {
		doc["rules"] = []any{"MATCH,Rayut"}
	}
}

func projectSelectors(doc map[string]any, links map[string]string, delays map[string]int) []SelectorInfo {
	fields := map[string]ProxyField{}
	for _, field := range projectProxies(doc) {
		fields[field.Name] = field
	}
	groups, _ := doc["proxy-groups"].([]any)
	out := make([]SelectorInfo, 0)
	active := effectiveNode(doc)
	traffic := matchTarget(doc)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok || !selectableKind(scalarText(group["type"])) {
			continue
		}
		now := scalarText(group["now"])
		manual := manualSelect(scalarText(group["type"]))
		list, _ := group["proxies"].([]any)
		nodes := make([]NodeInfo, 0, len(list))
		for _, entry := range list {
			nodeName := scalarText(entry)
			if nodeName == "" {
				continue
			}
			selected := manual && now != "" && nodeName == now
			if !manual && active != "" && nodeName == active {
				selected = true
			}
			info := NodeInfo{Index: -1, Name: nodeName, Selected: selected}
			if field, ok := fields[nodeName]; ok {
				info.Index = field.Index
				info.Type = field.Type
				info.Network = field.Network
				info.Delay = delays[nodeName]
				info.Shareable = strings.TrimSpace(links[nodeName]) != ""
			}
			nodes = append(nodes, info)
		}
		groupName := scalarText(group["name"])
		out = append(out, SelectorInfo{
			Name:       groupName,
			Type:       scalarText(group["type"]),
			Selectable: manual,
			Traffic:    groupName != "" && groupName == traffic,
			Now:        now,
			Nodes:      nodes,
		})
	}
	return out
}

// applyChoice writes the tapped node onto every manual group that lists it.
// A url-test tab cannot store a choice itself; the manual groups still follow.
func applyChoice(doc map[string]any, viewed, node string) error {
	targets := choiceTargets(doc, viewed, node)
	if len(targets) == 0 {
		if group := groupByName(doc, viewed); group != nil && !manualSelect(scalarText(group["type"])) {
			return errCode("not selectable")
		}
		return errCode("not found")
	}
	for _, target := range targets {
		group := groupByName(doc, target[0])
		if group != nil {
			group["now"] = target[1]
		}
	}
	return nil
}

func manualSelect(kind string) bool {
	return strings.EqualFold(kind, "select")
}

func proxyListed(group map[string]any, name string) bool {
	list, _ := group["proxies"].([]any)
	for _, item := range list {
		if scalarText(item) == name {
			return true
		}
	}
	return false
}

func matchTarget(doc map[string]any) string {
	rules, _ := doc["rules"].([]any)
	target := ""
	for _, item := range rules {
		text := strings.TrimSpace(scalarText(item))
		if !strings.HasPrefix(strings.ToUpper(text), "MATCH,") {
			continue
		}
		parts := strings.Split(text, ",")
		target = strings.TrimSpace(parts[len(parts)-1])
	}
	return target
}

func groupByName(doc map[string]any, name string) map[string]any {
	if name == "" {
		return nil
	}
	groups, _ := doc["proxy-groups"].([]any)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if ok && scalarText(group["name"]) == name {
			return group
		}
	}
	return nil
}

// effectiveNode is the node a new connection uses. It follows MATCH, then each
// group's now when that value names another group. An empty now does not guess.
func effectiveNode(doc map[string]any) string {
	name := matchTarget(doc)
	if name == "" {
		return selectedProxy(doc)
	}
	for i := 0; i < 6; i++ {
		group := groupByName(doc, name)
		if group == nil {
			return name
		}
		now := scalarText(group["now"])
		if now == "" || now == name {
			return ""
		}
		name = now
	}
	return ""
}

func choiceTargets(doc map[string]any, viewed, node string) [][2]string {
	groups, _ := doc["proxy-groups"].([]any)
	out := make([][2]string, 0)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok || !manualSelect(scalarText(group["type"])) {
			continue
		}
		groupName := scalarText(group["name"])
		if groupName == "" {
			continue
		}
		if proxyListed(group, node) {
			out = append(out, [2]string{groupName, node})
			continue
		}
		if viewed != "" && viewed != groupName && proxyListed(group, viewed) {
			out = append(out, [2]string{groupName, viewed})
		}
	}
	traffic := matchTarget(doc)
	for i, target := range out {
		if target[0] == traffic && i != 0 {
			out[0], out[i] = out[i], out[0]
			break
		}
	}
	return out
}

func selectedProxy(doc map[string]any) string {
	groups, _ := doc["proxy-groups"].([]any)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok || !selectableKind(scalarText(group["type"])) {
			continue
		}
		if now := scalarText(group["now"]); now != "" {
			return now
		}
	}
	return ""
}

func dropProxyName(doc map[string]any, name string) {
	if name == "" {
		return
	}
	groups, _ := doc["proxy-groups"].([]any)
	for _, item := range groups {
		group, ok := item.(map[string]any)
		if !ok {
			continue
		}
		list, _ := group["proxies"].([]any)
		kept := make([]any, 0, len(list))
		for _, entry := range list {
			if scalarText(entry) != name {
				kept = append(kept, entry)
			}
		}
		group["proxies"] = kept
		if scalarText(group["now"]) == name {
			if len(kept) > 0 {
				group["now"] = scalarText(kept[0])
			} else {
				delete(group, "now")
			}
		}
	}
}

func uniqueProxyName(doc map[string]any, name string) string {
	if strings.TrimSpace(name) == "" {
		name = "节点"
	}
	taken := map[string]bool{}
	for _, field := range projectProxies(doc) {
		taken[field.Name] = true
	}
	if !taken[name] {
		return name
	}
	for n := 2; ; n++ {
		next := name + " " + strconv.Itoa(n)
		if !taken[next] {
			return next
		}
	}
}

func selectableKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "select", "url-test", "fallback", "load-balance":
		return true
	default:
		return false
	}
}

func newGroupID() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "group"
	}
	return hex.EncodeToString(buf)
}

func (s *Store) indexPath() string            { return filepath.Join(s.Dir, "groups", "index.json") }
func (s *Store) groupDir(id string) string    { return filepath.Join(s.Dir, "groups", id) }
func (s *Store) groupConfig(id string) string { return filepath.Join(s.groupDir(id), "config.yaml") }
func (s *Store) groupURL(id string) string    { return filepath.Join(s.groupDir(id), "url") }
func (s *Store) groupLinks(id string) string  { return filepath.Join(s.groupDir(id), "links.json") }
func (s *Store) groupDelays(id string) string { return filepath.Join(s.groupDir(id), "delays.json") }
