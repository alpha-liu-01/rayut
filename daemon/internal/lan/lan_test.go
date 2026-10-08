package lan

import (
	"path/filepath"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

func TestMissingSwitchIsOff(t *testing.T) {
	old := paths.AllowLAN
	paths.AllowLAN = filepath.Join(t.TempDir(), "allow-lan")
	t.Cleanup(func() { paths.AllowLAN = old })
	if Enabled() {
		t.Fatal("missing file is on")
	}
	if err := Set(true); err != nil || !Enabled() {
		t.Fatal(err)
	}
	if err := Set(false); err != nil || Enabled() {
		t.Fatal(err)
	}
}
