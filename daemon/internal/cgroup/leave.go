package cgroup

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LeaveAppScope moves pid out of a Lomiri click cgroup. Otherwise swiping the
// app away leaves root processes in that cgroup and the app cannot start again.
func LeaveAppScope(pid int) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return err
	}
	traceLeave("cgroup " + strconv.Itoa(pid) + " " + strings.Join(strings.Fields(string(data)), " "))
	if pureV2() {
		// 24.04 has only the v2 hierarchy. The user manager cannot signal a
		// root process, so a helper left in the click scope keeps that cgroup
		// populated and the next launch never finishes. Join a user scope
		// under the same user slice. The click cgroup name is not always
		// visible at the first read, and the process can be put back after
		// sudo returns, so keep checking for a short while.
		// 20.04 still has /sys/fs/cgroup/unified and uses the parent write.
		watchClickScope(pid)
		return leavePureV2(pid)
	}
	var problems []string
	for _, line := range strings.Split(string(data), "\n") {
		controllers, path, ok := splitCgroup(line)
		if !ok || !strings.Contains(path, "lomiri-app-launch") {
			continue
		}
		if err := leaveV1(mountPoint(controllers), path, pid); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

var (
	watchMu sync.Mutex
	watched = map[int]bool{}
)

func watchClickScope(pid int) {
	watchMu.Lock()
	if watched[pid] {
		watchMu.Unlock()
		return
	}
	watched[pid] = true
	watchMu.Unlock()
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(300 * time.Millisecond)
			data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
			if err != nil || !strings.Contains(string(data), "lomiri-app-launch") {
				continue
			}
			if err := leavePureV2(pid); err != nil {
				traceLeave("rejoin " + strconv.Itoa(pid) + " " + err.Error())
			}
		}
	}()
}

func splitCgroup(line string) (controllers, path string, ok bool) {
	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, ":", 3)
	if len(parts) != 3 || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// statPath is replaced by tests. Production uses os.Stat.
var statPath = os.Stat

func pureV2() bool {
	_, err := statPath("/sys/fs/cgroup/unified")
	return err != nil
}

func mountPoint(controllers string) string {
	switch {
	case controllers == "":
		// 20.04 mounts the v2 hierarchy beside the v1 trees. 24.04 has only
		// the v2 hierarchy, directly at /sys/fs/cgroup.
		if _, err := statPath("/sys/fs/cgroup/unified"); err == nil {
			return "/sys/fs/cgroup/unified"
		}
		return "/sys/fs/cgroup"
	case strings.Contains(controllers, "name=systemd"):
		return "/sys/fs/cgroup/systemd"
	default:
		name, _, _ := strings.Cut(controllers, ",")
		return "/sys/fs/cgroup/" + name
	}
}

func parentPath(cgroupPath string) string {
	parent := cgroupPath[:strings.LastIndex(cgroupPath, "/")]
	if parent == "" {
		return "/"
	}
	return parent
}

// Note appends one line to /tmp/rayutd-cgroup.log. The click user can read it.
func Note(message string) {
	traceLeave(message)
}

func traceLeave(message string) {
	file, err := os.OpenFile("/tmp/rayutd-cgroup.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintln(file, message)
	file.Close()
}

var userInPath = regexp.MustCompile(`(?:^|/)user-([0-9]+)\.slice(?:/|$)`)

func userFromPath(cgroupPath string) string {
	match := userInPath.FindStringSubmatch(cgroupPath)
	if match == nil {
		return ""
	}
	return match[1]
}

func helperScopeProcs(uid string) string {
	return "/sys/fs/cgroup/user.slice/user-" + uid + ".slice/user@" + uid + ".service/app.slice/rayut-helper.scope/cgroup.procs"
}

func helperScopeArgs() []string {
	return []string{
		"--user", "--scope",
		"--unit=rayut-helper.scope",
		"--collect",
		"--property=Delegate=yes",
		"--property=KillMode=none",
		"/bin/sleep", "infinity",
	}
}

func clickUser(cgroupText string) string {
	if id := strings.TrimSpace(os.Getenv("SUDO_UID")); id != "" && id != "0" {
		return id
	}
	if data, err := os.ReadFile(loginUIDPath); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" && id != "0" && id != "4294967295" {
			return id
		}
	}
	for _, line := range strings.Split(cgroupText, "\n") {
		_, path, ok := splitCgroup(line)
		if !ok {
			continue
		}
		if uid := userFromPath(path); uid != "" {
			return uid
		}
	}
	return ""
}

func leavePureV2(pid int) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return err
	}
	uid := clickUser(string(data))
	if uid == "" {
		return fmt.Errorf("cgroup user missing")
	}
	name, gid, home, ok := clickIdentity(uid)
	if !ok {
		return fmt.Errorf("click user missing")
	}
	if err := ensureHelperScope(uid, gid, name, home); err != nil {
		traceLeave("scope " + err.Error())
		return err
	}
	if err := writePID(helperScopeProcs(uid), pid); err != nil {
		traceLeave("scope join " + err.Error())
		return err
	}
	if after, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid)); err == nil {
		traceLeave("after " + strconv.Itoa(pid) + " " + strings.Join(strings.Fields(string(after)), " "))
	}
	traceLeave("v2 scope " + strconv.Itoa(pid))
	return nil
}

// loginUIDPath, userRuntimeRoot, and passwdFiles are replaced by tests.
var (
	loginUIDPath    = "/proc/self/loginuid"
	userRuntimeRoot = "/run/user"
	passwdFiles     = []string{"/etc/passwd", "/var/lib/extrausers/passwd"}
)

// clickIdentity names the click user without reading /etc/passwd.
// On Ubuntu Touch that account lives in extrausers, and the runtime
// directory /run/user/<uid> is already owned by the account's group.
func clickIdentity(uid string) (name, gid, home string, ok bool) {
	info, err := os.Stat(userRuntimeRoot + "/" + uid)
	if err == nil {
		if st, isStat := info.Sys().(*syscall.Stat_t); isStat && st.Gid != 0 {
			return uid, strconv.FormatUint(uint64(st.Gid), 10), userRuntimeRoot + "/" + uid, true
		}
	}
	return lookupPasswd(uid)
}

func lookupPasswd(uid string) (name, gid, home string, ok bool) {
	if out, err := exec.Command("getent", "passwd", uid).Output(); err == nil {
		if name, gid, home, ok = parsePasswdRecord(string(out), uid); ok {
			return
		}
	}
	for _, path := range passwdFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if name, gid, home, ok = parsePasswdRecord(line, uid); ok {
				return
			}
		}
	}
	return "", "", "", false
}

func parsePasswdRecord(line, uid string) (name, gid, home string, ok bool) {
	fields := strings.Split(strings.TrimSpace(line), ":")
	if len(fields) < 6 || fields[2] != uid {
		return "", "", "", false
	}
	return fields[0], fields[3], fields[5], true
}

var scopeMu sync.Mutex

func ensureHelperScope(uid, gid, name, home string) error {
	scopeMu.Lock()
	defer scopeMu.Unlock()
	dir := strings.TrimSuffix(helperScopeProcs(uid), "/cgroup.procs")
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return nil
	}
	uidN, err := strconv.Atoi(uid)
	if err != nil {
		return err
	}
	gidN, err := strconv.Atoi(gid)
	if err != nil {
		return err
	}
	env := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"XDG_RUNTIME_DIR=/run/user/" + uid,
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + uid + "/bus",
		"HOME=" + home,
		"USER=" + name,
		"LOGNAME=" + name,
	}
	cred := func() *syscall.SysProcAttr {
		return &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uidN), Gid: uint32(gidN)}}
	}
	reset := exec.Command("systemctl", "--user", "reset-failed", "rayut-helper.scope")
	reset.Env = env
	reset.SysProcAttr = cred()
	_ = reset.Run()
	cmd := exec.Command("systemd-run", helperScopeArgs()...)
	cmd.Env = env
	cmd.SysProcAttr = cred()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("helper scope missing")
}

func leaveV1(mount, cgroupPath string, pid int) error {
	parent := parentPath(cgroupPath)
	if err := writePID(mount+parent+"/cgroup.procs", pid); err != nil {
		return writePID(mount+"/cgroup.procs", pid)
	}
	return nil
}

func writePID(path string, pid int) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = file.WriteString(strconv.Itoa(pid) + "\n")
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
