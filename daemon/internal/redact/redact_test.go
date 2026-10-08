package redact

import (
	"strings"
	"testing"
)

func TestSampleDropsCredentials(t *testing.T) {
	const sample = `
time="2026-10-07T21:00:00.000000000-04:00" level=info msg="Tun adapter listening at Meta"
proxies:
- name: lab
  password: super-secret-password
  uuid: 11111111-2222-3333-4444-555555555555
  public-key: reality-public-key
  private-key: reality-private-key
  short-id: realityshort
  url: https://airport.example/sub?token=sub-token-value
{"password":"super-secret-password","uuid":"11111111-2222-3333-4444-555555555555","public-key":"reality-public-key"}
Authorization: Bearer controller-token-value
vless://user:reality-private-key@example.com:443
destination example.com:443
`
	got := Text(sample)
	for _, secret := range []string{
		"super-secret-password",
		"11111111-2222-3333-4444-555555555555",
		"reality-public-key",
		"reality-private-key",
		"realityshort",
		"sub-token-value",
		"controller-token-value",
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %s remained", secret)
		}
	}
	if !strings.Contains(got, "Tun adapter listening at Meta") {
		t.Fatalf("ordinary line was removed: %s", got)
	}
	if !strings.Contains(got, "example.com:443") {
		t.Fatalf("destination was removed: %s", got)
	}
}
