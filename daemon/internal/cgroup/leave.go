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
		parent := path[:strings.LastIndex(path, "/")]
		if parent == "" {
			parent = "/"
		}
		if err := writePID(mount+parent+"/cgroup.procs", pid); err != nil {
			if err = writePID(mount+"/cgroup.procs", pid); err != nil {
				problems = append(problems, err.Error())
			}
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

func mountPoint(controllers string) string {
	switch {
	case controllers == "":
		return "/sys/fs/cgroup/unified"
	case strings.Contains(controllers, "name=systemd"):
		return "/sys/fs/cgroup/systemd"
	default:
		name, _, _ := strings.Cut(controllers, ",")
		return "/sys/fs/cgroup/" + name
	}
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
