package core

import (
	"os"
	"strings"
	"testing"
)

func TestShippedGPLMatchesLicense(t *testing.T) {
	if readRepo(t, "LICENSE") != readRepo(t, "app/notices/gpl-3.0.txt") {
		t.Fatal("app/notices/gpl-3.0.txt differs from LICENSE")
	}
}

func TestComponentNoticesNameEveryBinary(t *testing.T) {
	rel, ok := Lookup("v1.19.32")
	if !ok {
		t.Fatal("catalog missing v1.19.32")
	}
	en := readNotice(t, "components.en.txt")
	zh := readNotice(t, "components.zh.txt")
	for _, text := range []string{en, zh} {
		for _, field := range []string{rel.Tag, rel.License, rel.Source, rel.URL, rel.SHA256, rel.LicenseURL, rel.LicenseSHA256} {
			if !strings.Contains(text, field) {
				t.Errorf("component notice missing %s", field)
			}
		}
	}
	requireBlocks(t, en, "Binary: ")
	requireBlocks(t, zh, "二进制：")
}

func TestPrivilegeNoticeMatchesLauncher(t *testing.T) {
	commands := []string{
		"/usr/bin/sudo -S -p '' -- <application directory>/bin/rayutd --session",
		"ip rule show",
		"ip -6 rule show",
		"ip rule del pref <priority>",
		"ip -6 rule del pref <priority>",
		"ip route flush table 2022",
		"ip -6 route flush table 2022",
		"ip -6 route show table 2022",
		"ip -6 neigh replace <address> dev Meta nud permanent",
		"ip -o link show",
		"getent passwd <name>",
		"<mihomo> -v",
		"<mihomo> -t -d <check directory> -f <config>",
		"<mihomo> -d <runtime directory> -f <config>",
	}
	for _, name := range []string{"privileges.en.txt", "privileges.zh.txt"} {
		text := readNotice(t, name)
		for _, command := range commands {
			if !strings.Contains(text, command) {
				t.Errorf("%s missing %s", name, command)
			}
		}
	}
	launcher := readRepo(t, "app/plugins/Rayut/controller.cpp")
	for _, piece := range []string{
		`QStringLiteral("/usr/bin/sudo")`,
		`QStringLiteral("-S")`,
		`QStringLiteral("-p")`,
		`QStringLiteral("--")`,
		`QStringLiteral("--session")`,
	} {
		if !strings.Contains(launcher, piece) {
			t.Errorf("launcher missing %s", piece)
		}
	}
	recover := readRepo(t, "daemon/internal/route/recover.go")
	for _, piece := range []string{
		`"rule", "show"`,
		`"-6", "rule", "show"`,
		`"rule", "del", "pref"`,
		`"-6", "rule", "del", "pref"`,
		`"route", "flush", "table", "2022"`,
		`"-6", "route", "flush", "table", "2022"`,
		`"-6", "route", "show", "table", "2022"`,
		`"neigh", "replace"`,
		`"dev", "Meta", "nud", "permanent"`,
		`"-o", "link", "show"`,
	} {
		if !strings.Contains(recover, piece) {
			t.Errorf("recover missing %s", piece)
		}
	}
	mihomo := readRepo(t, "daemon/internal/core/mihomo.go")
	for _, piece := range []string{`"-v"`, `"-t"`, `"-d"`, `"-f"`} {
		if !strings.Contains(mihomo, piece) {
			t.Errorf("mihomo launcher missing %s", piece)
		}
	}
	if !strings.Contains(readRepo(t, "daemon/internal/paths/paths.go"), `"getent", "passwd"`) {
		t.Fatal("getent passwd missing")
	}
}

func requireBlocks(t *testing.T, text, marker string) {
	t.Helper()
	parts := strings.Split(text, marker)
	if len(parts) < 4 {
		t.Fatalf("expected at least 3 binaries, found %d", len(parts)-1)
	}
	version := "Version: "
	license := "License: "
	source := "Source: "
	if marker == "二进制：" {
		version = "版本："
		license = "许可证："
		source = "来源："
	}
	for _, part := range parts[1:4] {
		for _, label := range []string{version, license, source} {
			if !strings.Contains(part, label) {
				t.Errorf("binary block missing %s", label)
			}
		}
	}
}

func readNotice(t *testing.T, name string) string {
	t.Helper()
	return readRepo(t, "app/notices/"+name)
}

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("../../../" + rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
