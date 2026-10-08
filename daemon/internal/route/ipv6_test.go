package route

import "testing"

func TestIPv6GatewaysOnlyMetaVia(t *testing.T) {
	show := "" +
		"default via fdfe:dcba:9877::2 dev Meta metric 1024 pref medium\n" +
		"fdfe:dcba:9877::/126 dev Meta proto kernel metric 256 pref medium\n" +
		"default via fdfe:dcba:9877::2 dev Meta metric 512\n" +
		"2001:db8::/32 via fe80::1 dev wlan0 metric 100\n"
	got := IPv6Gateways(show, "Meta")
	if len(got) != 1 || got[0] != "fdfe:dcba:9877::2" {
		t.Fatalf("gateways: %#v", got)
	}
}
