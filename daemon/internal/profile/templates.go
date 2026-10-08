package profile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Built-in rule templates. They are authored in this file and are not
// downloaded at runtime. ${PROXY} is replaced with the profile's existing
// MATCH target, or the first proxy group name.

var lanRules = []string{
	"IP-CIDR,0.0.0.0/8,DIRECT,no-resolve",
	"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
	"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
	"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
	"IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
	"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
	"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
	"IP-CIDR,224.0.0.0/4,DIRECT,no-resolve",
	"IP-CIDR6,::1/128,DIRECT,no-resolve",
	"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
	"IP-CIDR6,fe80::/10,DIRECT,no-resolve",
	"DOMAIN-SUFFIX,local,DIRECT",
	"DOMAIN-SUFFIX,lan,DIRECT",
}

var chinaRules = []string{
	"DOMAIN-SUFFIX,cn,DIRECT",
	"DOMAIN-SUFFIX,com.cn,DIRECT",
	"DOMAIN-SUFFIX,net.cn,DIRECT",
	"DOMAIN-SUFFIX,org.cn,DIRECT",
	"DOMAIN-SUFFIX,baidu.com,DIRECT",
	"DOMAIN-SUFFIX,qq.com,DIRECT",
	"DOMAIN-SUFFIX,taobao.com,DIRECT",
	"DOMAIN-SUFFIX,tmall.com,DIRECT",
	"DOMAIN-SUFFIX,alipay.com,DIRECT",
	"DOMAIN-SUFFIX,aliyun.com,DIRECT",
	"DOMAIN-SUFFIX,bilibili.com,DIRECT",
	"DOMAIN-SUFFIX,163.com,DIRECT",
	"DOMAIN-SUFFIX,126.com,DIRECT",
	"DOMAIN-SUFFIX,weibo.com,DIRECT",
	"DOMAIN-SUFFIX,jd.com,DIRECT",
	"DOMAIN-SUFFIX,zhihu.com,DIRECT",
	"DOMAIN-SUFFIX,douyin.com,DIRECT",
	"DOMAIN-SUFFIX,iqiyi.com,DIRECT",
	"DOMAIN-SUFFIX,youku.com,DIRECT",
	"DOMAIN-SUFFIX,sohu.com,DIRECT",
	"DOMAIN-SUFFIX,cctv.com,DIRECT",
	"DOMAIN-SUFFIX,xinhuanet.com,DIRECT",
}

func templateRules(id, policy string) ([]string, bool) {
	var body []string
	switch id {
	case "global":
	case "lan":
		body = append(body, lanRules...)
	case "lan-china":
		body = append(body, lanRules...)
		body = append(body, chinaRules...)
	default:
		return nil, false
	}
	out := make([]string, 0, len(body)+1)
	out = append(out, body...)
	out = append(out, "MATCH,"+policy)
	return out, true
}

func proxyPolicy(doc map[string]any) string {
	if rules, ok := doc["rules"].([]any); ok {
		for i := len(rules) - 1; i >= 0; i-- {
			if policy, ok := matchPolicy(ruleText(rules[i])); ok {
				return policy
			}
		}
	}
	if groups, ok := doc["proxy-groups"].([]any); ok {
		for _, item := range groups {
			group, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := ruleText(group["name"])
			if name != "" && name != "<nil>" {
				return name
			}
		}
	}
	return "Rayut"
}

func ruleText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func matchPolicy(rule string) (string, bool) {
	parts := strings.Split(rule, ",")
	if len(parts) < 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "MATCH") {
		return "", false
	}
	policy := strings.TrimSpace(parts[1])
	if policy == "" {
		return "", false
	}
	return policy, true
}

func (s *Store) ApplyTemplate(id string) (View, error) {
	if s.TunUp != nil && s.TunUp() {
		return s.View(), errCode("tun running")
	}
	raw, err := os.ReadFile(s.activePath())
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		return s.View(), errCode("profile missing")
	}
	var doc map[string]any
	if yaml.Unmarshal(raw, &doc) != nil || len(doc) == 0 {
		return s.View(), errCode("invalid yaml")
	}
	rules, ok := templateRules(id, proxyPolicy(doc))
	if !ok {
		return s.View(), errCode("unknown template")
	}
	applied := make([]any, 0, len(rules))
	for _, rule := range rules {
		applied = append(applied, rule)
	}
	doc["rules"] = applied
	out, err := yaml.Marshal(doc)
	if err != nil {
		return s.View(), errCode("invalid yaml")
	}
	if err := os.MkdirAll(s.checkDir(), 0o700); err != nil {
		return View{}, err
	}
	if err := s.stagePayload(); err != nil {
		return View{}, err
	}
	temp, err := os.CreateTemp(s.checkDir(), "template-*.yaml")
	if err != nil {
		return View{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(out); err != nil {
		temp.Close()
		return View{}, err
	}
	if err := temp.Close(); err != nil {
		return View{}, err
	}
	if s.Test == nil {
		return s.View(), errCode("core missing")
	}
	if err := s.Test(tempName); err != nil {
		if err.Error() == "core missing" {
			return s.View(), errCode("core missing")
		}
		return s.View(), errCode("invalid config")
	}
	if err := s.installActive(out); err != nil {
		return View{}, err
	}
	state := s.readState()
	state.RuleTemplate = id
	state.Error = ""
	if err := s.writeState(state); err != nil {
		return View{}, err
	}
	return s.View(), nil
}

func (s *Store) stagePayload() error {
	body, err := os.ReadFile(s.payloadPath())
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return s.atomic(filepath.Join(s.checkDir(), "subscription.payload"), body)
}
