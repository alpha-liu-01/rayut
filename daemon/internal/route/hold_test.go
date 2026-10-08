package route

import "testing"

func TestOwnsAny(t *testing.T) {
	if OwnsAny("0:\tfrom all lookup local\n32766:\tfrom all lookup main\n") {
		t.Fatal("system rules are not owned")
	}
	if !OwnsAny("9002:\tfrom all lookup 2022\n") {
		t.Fatal("lookup 2022 is owned")
	}
	if OwnsAny("9002:\tfrom all lookup main\n") {
		t.Fatal("unexpected rule is not owned")
	}
}
