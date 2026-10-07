package cgroup

import "testing"

func TestSplitCgroup(t *testing.T) {
	controllers, path, ok := splitCgroup("0::/user.slice/user-32011.slice/lomiri-app-launch--rayut.service")
	if !ok || controllers != "" || path != "/user.slice/user-32011.slice/lomiri-app-launch--rayut.service" {
		t.Fatalf("v2: %q %q %v", controllers, path, ok)
	}
	controllers, path, ok = splitCgroup("1:name=systemd:/user.slice/lomiri-app-launch--rayut.service")
	if !ok || controllers != "name=systemd" || path == "" {
		t.Fatalf("systemd: %q %q %v", controllers, path, ok)
	}
}

func TestMountPoint(t *testing.T) {
	if mountPoint("") != "/sys/fs/cgroup/unified" {
		t.Fatal("unified")
	}
	if mountPoint("name=systemd") != "/sys/fs/cgroup/systemd" {
		t.Fatal("systemd")
	}
}
