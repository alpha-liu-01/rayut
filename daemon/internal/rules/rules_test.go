package rules

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		line string
		prio string
		kind string
	}{
		{"0:\tfrom all lookup local", "", "ignore"},
		{"32766:\tfrom all lookup main", "", "ignore"},
		{"9000:\tfrom all to 198.18.0.1/30 lookup 2022", "9000", "owned"},
		{"9001:\tnot from all lookup main suppress_prefixlength 0", "9001", "owned"},
		{"9001:\tfrom all iif Meta [detached] goto 9010", "9001", "owned"},
		{"9000:\tfrom ::/1 iif lo goto 9010", "9000", "owned"},
		{"9000:\tfrom 8000::/1 iif lo goto 9010", "9000", "owned"},
		{"9001:\tfrom fdfe:dcba:9876::/126 iif lo lookup 2022", "9001", "owned"},
		{"9002:\tfrom all lookup 2022", "9002", "owned"},
		{"9010:\tfrom all nop", "9010", "owned"},
		{"9002:\tfrom all lookup main", "9002", "unexpected"},
	}
	for _, tc := range cases {
		prio, kind := Classify(tc.line)
		if prio != tc.prio || kind != tc.kind {
			t.Fatalf("%q: got %s %s, want %s %s", tc.line, prio, kind, tc.prio, tc.kind)
		}
	}
}
