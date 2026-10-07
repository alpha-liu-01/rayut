package paths

import "testing"

func TestNumericPair(t *testing.T) {
	uid, gid, ok := numericPair("32011", "32011")
	if !ok || uid != 32011 || gid != 32011 {
		t.Fatalf("got %d %d %v", uid, gid, ok)
	}
	if _, _, ok := numericPair("phablet", "32011"); ok {
		t.Fatal("expected non-numeric uid to fail")
	}
}
