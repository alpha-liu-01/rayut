package cgroup

import (
	"os"
	"testing"
)

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

func TestPureV2(t *testing.T) {
	saved := statPath
	t.Cleanup(func() { statPath = saved })
	statPath = func(name string) (os.FileInfo, error) {
		if name == "/sys/fs/cgroup/unified" {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}
	if pureV2() {
		t.Fatal("hybrid should still move the unified hierarchy")
	}
	statPath = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	if !pureV2() {
		t.Fatal("pure v2 should stay in the click scope")
	}
}

func TestParentPath(t *testing.T) {
	got := parentPath("/user.slice/user-32011.slice/user@32011.service/app.slice/lomiri-app-launch--rayut.service")
	want := "/user.slice/user-32011.slice/user@32011.service/app.slice"
	if got != want {
		t.Fatalf("parent: %s", got)
	}
}

func TestMountPoint(t *testing.T) {
	saved := statPath
	t.Cleanup(func() { statPath = saved })

	statPath = func(name string) (os.FileInfo, error) {
		if name == "/sys/fs/cgroup/unified" {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}
	if mountPoint("") != "/sys/fs/cgroup/unified" {
		t.Fatal("hybrid v2 mount")
	}
	statPath = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	if mountPoint("") != "/sys/fs/cgroup" {
		t.Fatal("pure v2 mount")
	}
	if mountPoint("name=systemd") != "/sys/fs/cgroup/systemd" {
		t.Fatal("systemd")
	}
}
