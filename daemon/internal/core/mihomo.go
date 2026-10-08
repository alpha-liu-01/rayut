package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/cgroup"
	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

var (
	versionOnce sync.Once
	versionText = "unknown"
)

// Version runs `mihomo -v` once and returns the version token.
// `mihomo version` is not a version flag and must not be used.
func Version() string {
	versionOnce.Do(func() {
		out, err := exec.Command(paths.Mihomo, "-v").CombinedOutput()
		text := strings.TrimSpace(string(out))
		if text == "" {
			if err != nil {
				versionText = "unknown"
			}
			return
		}
		for _, field := range strings.Fields(text) {
			if strings.HasPrefix(field, "v") && strings.Contains(field, ".") {
				versionText = field
				return
			}
		}
		versionText = strings.Split(text, "\n")[0]
	})
	return versionText
}

func pidFile() string { return paths.Runtime + "/mihomo.pid" }
func logFile() string { return paths.Runtime + "/mihomo.log" }

func Alive() (int, bool) {
	data, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return 0, false
	}
	return pid, true
}

// Test checks a config with the fixed mihomo binary and discards its output.
func Test(configPath string) error {
	if _, err := os.Stat(paths.Mihomo); err != nil {
		return fmt.Errorf("core missing")
	}
	dir := filepath.Join(paths.Runtime, "check")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, paths.Mihomo, "-t", "-d", dir, "-f", configPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid config")
	}
	return nil
}

// Start launches the fixed mihomo binary. The caller recovers routes first.
func Start() error {
	if _, ok := Alive(); ok {
		return fmt.Errorf("already-running")
	}
	if _, err := os.Stat(paths.Mihomo); err != nil {
		return fmt.Errorf("core missing")
	}
	if _, err := os.Stat(paths.Profile); err != nil {
		return fmt.Errorf("profile missing")
	}
	if err := os.MkdirAll(paths.Runtime, 0o700); err != nil {
		return err
	}
	payload := filepath.Join(filepath.Dir(paths.Profile), "subscription.payload")
	if data, err := os.ReadFile(payload); err == nil {
		if err := os.WriteFile(filepath.Join(paths.Runtime, "subscription.payload"), data, 0o600); err != nil {
			return err
		}
	}
	log, err := openSessionLog()
	if err != nil {
		return err
	}
	configPath, err := prepareRunConfig()
	if err != nil {
		log.Close()
		return err
	}
	cmd := exec.Command(paths.Mihomo, "-d", paths.Runtime, "-f", configPath)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		log.Close()
		return err
	}
	if err := os.WriteFile(pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o600); err != nil {
		_ = cmd.Process.Kill()
		log.Close()
		return err
	}
	if err := cgroup.LeaveAppScope(cmd.Process.Pid); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	go func() {
		_ = cmd.Wait()
		log.Close()
	}()
	return nil
}

// WaitTun returns when the log says the adapter is listening.
func WaitTun(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logFile())
		if err == nil && strings.Contains(string(data), "Tun adapter listening") {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("tun did not start")
}

// Stop sends SIGTERM to the recorded mihomo process.
func Stop() error {
	pid, ok := Alive()
	if !ok {
		return nil
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := Alive(); !ok {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("mihomo did not exit")
}
