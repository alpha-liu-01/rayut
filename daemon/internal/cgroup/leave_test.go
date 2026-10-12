package cgroup

import (
	"os"
	"strings"
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
		t.Fatal("pure v2 should be detected")
	}
}

func TestUserFromPath(t *testing.T) {
	got := userFromPath("/user.slice/user-32011.slice/user@32011.service/app.slice/lomiri-app-launch--rayut.service")
	if got != "32011" {
		t.Fatalf("uid %q", got)
	}
	if userFromPath("/system.slice/foo.service") != "" {
		t.Fatal("expected no uid")
	}
}

func TestHelperScopeProcs(t *testing.T) {
	got := helperScopeProcs("32011")
	want := "/sys/fs/cgroup/user.slice/user-32011.slice/user@32011.service/app.slice/rayut-helper.scope/cgroup.procs"
	if got != want {
		t.Fatal(got)
	}
	args := strings.Join(helperScopeArgs(), " ")
	for _, piece := range []string{"--user", "--scope", "--unit=rayut-helper.scope", "--collect", "--property=Delegate=yes", "--property=KillMode=none", "/bin/sleep", "infinity"} {
		if !strings.Contains(args, piece) {
			t.Fatalf("args missing %s: %s", piece, args)
		}
	}
}

func TestClickUser(t *testing.T) {
	t.Setenv("SUDO_UID", "32011")
	if got := clickUser("0::/"); got != "32011" {
		t.Fatalf("sudo uid %q", got)
	}
	t.Setenv("SUDO_UID", "")
	saved := loginUIDPath
	t.Cleanup(func() { loginUIDPath = saved })
	loginUIDPath = t.TempDir() + "/missing"
	got := clickUser("0::/user.slice/user-32011.slice/user@32011.service/app.slice/lomiri-app-launch--rayut.service")
	if got != "32011" {
		t.Fatalf("path uid %q", got)
	}
}

func TestClickIdentityUsesRuntimeDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(root+"/424242", 0o755); err != nil {
		t.Fatal(err)
	}
	savedRoot := userRuntimeRoot
	savedFiles := passwdFiles
	t.Cleanup(func() {
		userRuntimeRoot = savedRoot
		passwdFiles = savedFiles
	})
	userRuntimeRoot = root
	passwdFiles = nil
	name, gid, home, ok := clickIdentity("424242")
	if !ok || name != "424242" || gid == "" || gid == "0" || home != root+"/424242" {
		t.Fatalf("%v %s %s %s", ok, name, gid, home)
	}
}

func TestLookupPasswdReadsExtrausers(t *testing.T) {
	dir := t.TempDir()
	extra := dir + "/extrausers"
	body := "phablet:x:424242:424242:,,,:/home/phablet:/bin/bash\n"
	if err := os.WriteFile(extra, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	savedRoot := userRuntimeRoot
	savedFiles := passwdFiles
	t.Cleanup(func() {
		userRuntimeRoot = savedRoot
		passwdFiles = savedFiles
	})
	userRuntimeRoot = dir + "/missing"
	passwdFiles = []string{dir + "/passwd", extra}
	name, gid, home, ok := clickIdentity("424242")
	if !ok || name != "phablet" || gid != "424242" || home != "/home/phablet" {
		t.Fatalf("%v %s %s %s", ok, name, gid, home)
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
