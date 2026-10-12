package cgroup

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LeaveAppScope moves pid out of a Lomiri click cgroup. Otherwise swiping the
// app away leaves root processes in that cgroup and the app cannot start again.
func LeaveAppScope(pid int) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return err
	}
	var problems []string
	for _, line := range strings.Split(string(data), "\n") {
		controllers, path, ok := splitCgroup(line)
		if !ok || !strings.Contains(path, "lomiri-app-launch") {
			continue
		}
		mount := mountPoint(controllers)
		var err error
		if controllers == "" && pureV2() {
			// 24.04 has only the v2 hierarchy, and app.slice Delegate=no.
			// A neighboring cgroup is removed with the process inside it, and
			// the cgroup root is cleared when the sudo session closes. Stay in
			// the click scope. 20.04 still has /sys/fs/cgroup/unified beside
			// the v1 trees, and that line is moved with the v1 parent write.
			traceLeave("v2 stay " + strconv.Itoa(pid))
		} else {
			err = leaveV1(mount, path, pid)
		}
		if err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
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
