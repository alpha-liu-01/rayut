package killswitch

import "testing"

func TestObserve(t *testing.T) {
	var watch Watch
	if watch.Observe(true, false, false) || watch.Blocked || !watch.WasUp {
		t.Fatal("startup running")
	}
	if watch.Observe(false, true, true) || watch.Blocked {
		t.Fatal("intentional stop must not hold")
	}
	watch.WasUp = true
	if !watch.Observe(false, false, false) || watch.Blocked {
		t.Fatal("switch off should recover")
	}
	watch.WasUp = true
	if watch.Observe(false, false, true) || !watch.Blocked {
		t.Fatal("switch on should hold")
	}
	if watch.Observe(false, false, true) || !watch.Blocked {
		t.Fatal("holding twice")
	}
	if watch.Observe(true, false, true) || watch.Blocked || !watch.WasUp {
		t.Fatal("reconnect clears the hold")
	}
}
