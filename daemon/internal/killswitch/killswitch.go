package killswitch

import (
	"os"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

// Enabled reports the explicit kill switch. A missing file means off.
func Enabled() bool {
	data, err := os.ReadFile(paths.KillSwitch)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// Set stores the switch. The file has no node data and no secrets.
func Set(on bool) error {
	value := "0\n"
	if on {
		value = "1\n"
	}
	return os.WriteFile(paths.KillSwitch, []byte(value), 0o600)
}

// Watch remembers whether the core was up so a later exit can be classified.
type Watch struct {
	WasUp   bool
	Blocked bool
}

// Observe handles one sample. A normal stop does nothing here; the caller
// recovers. An unexpected exit recovers only when the switch is off.
func (w *Watch) Observe(alive, stopping, enabled bool) (shouldRecover bool) {
	if w.WasUp && !alive && !stopping {
		if enabled {
			w.Blocked = true
		} else {
			w.Blocked = false
			shouldRecover = true
		}
	} else if alive {
		w.Blocked = false
	}
	w.WasUp = alive
	return shouldRecover
}
