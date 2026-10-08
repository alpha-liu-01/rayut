package paths

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const appID = "rayut.rayut"

var (
	Mihomo      string
	Runtime     string
	Profile     string
	ClientToken string
	Ready       string
	HelperPid   string
	KillSwitch  string
	AllowLAN    string
	uid         int
	gid         int
)

// Init resolves the Click-relative core and the per-user data directories.
// The data directories match QStandardPaths for application name rayut.rayut:
// ~/.local/share/rayut.rayut and ~/.config/rayut.rayut.
func Init() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	Mihomo = filepath.Join(filepath.Dir(exe), "mihomo")
	home, ownerUID, ownerGID, err := invokingHome()
	if err != nil {
		return err
	}
	uid, gid = ownerUID, ownerGID
	data := filepath.Join(home, ".local", "share", appID)
	config := filepath.Join(home, ".config", appID)
	Runtime = filepath.Join(data, "runtime")
	Profile = filepath.Join(data, "profiles", "active.yaml")
	ClientToken = filepath.Join(config, "client-token")
	Ready = filepath.Join(Runtime, "ready")
	HelperPid = filepath.Join(Runtime, "rayutd.pid")
	KillSwitch = filepath.Join(data, "kill-switch")
	AllowLAN = filepath.Join(data, "allow-lan")
	return prepareDirs(data, config)
}

func Owner() (int, int) {
	return uid, gid
}

func invokingHome() (string, int, int, error) {
	userName := os.Getenv("SUDO_USER")
	if userName == "" {
		userName = "phablet"
	}
	uidText := os.Getenv("SUDO_UID")
	gidText := os.Getenv("SUDO_GID")
	out, err := exec.Command("getent", "passwd", userName).Output()
	if err != nil {
		return "", 0, 0, err
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(fields) < 6 || fields[5] == "" {
		return "", 0, 0, fmt.Errorf("unexpected getent passwd output")
	}
	if uidText == "" {
		uidText = fields[2]
	}
	if gidText == "" {
		gidText = fields[3]
	}
	ownerUID, ownerGID, ok := numericPair(uidText, gidText)
	if !ok {
		return "", 0, 0, fmt.Errorf("unexpected getent passwd output")
	}
	return fields[5], ownerUID, ownerGID, nil
}

func prepareDirs(data, config string) error {
	if err := os.MkdirAll(filepath.Join(data, "profiles"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(Runtime, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(config, 0o700); err != nil {
		return err
	}
	if err := os.Chown(data, uid, gid); err != nil {
		return err
	}
	if err := os.Chown(filepath.Join(data, "profiles"), uid, gid); err != nil {
		return err
	}
	if err := os.Chown(config, uid, gid); err != nil {
		return err
	}
	return os.Chown(Runtime, 0, 0)
}

func numericPair(uidText, gidText string) (int, int, bool) {
	parsedUID, err := strconv.Atoi(uidText)
	if err != nil {
		return 0, 0, false
	}
	parsedGID, err := strconv.Atoi(gidText)
	if err != nil {
		return 0, 0, false
	}
	return parsedUID, parsedGID, true
}
