package lan

import (
	"os"
	"strings"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

// Enabled reports the LAN switch. A missing file means off.
func Enabled() bool {
	data, err := os.ReadFile(paths.AllowLAN)
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
	return os.WriteFile(paths.AllowLAN, []byte(value), 0o600)
}
