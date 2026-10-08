package core

import (
	"os"
	"strings"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

func TestSessionLogDropsCredentialsOnDisk(t *testing.T) {
	dir := t.TempDir()
	old := paths.Runtime
	paths.Runtime = dir
	t.Cleanup(func() { paths.Runtime = old })

	writer, err := openSessionLog()
	if err != nil {
		t.Fatal(err)
	}
	const sample = "time=\"2026-10-07T21:00:00.000000000-04:00\" level=info msg=\"Tun adapter listening at Meta\"\n" +
		"password: super-secret-password uuid: 11111111-2222-3333-4444-555555555555 public-key: reality-public-key private-key: reality-private-key short-id: realityshort\n" +
		"https://airport.example/sub?token=sub-token-value\n" +
		"Authorization: Bearer controller-token-value\n"
	if _, err := writer.Write([]byte(sample)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	stored, err := os.ReadFile(logFile())
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecrets(t, string(stored))
	if !strings.Contains(string(stored), "Tun adapter listening at Meta") {
		t.Fatalf("disk log lost the ordinary line: %s", stored)
	}

	lines, err := SessionLog()
	if err != nil {
		t.Fatal(err)
	}
	var joined strings.Builder
	for _, line := range lines {
		joined.WriteString(line.Type)
		joined.WriteByte(' ')
		joined.WriteString(line.Payload)
		joined.WriteByte('\n')
	}
	assertNoSecrets(t, joined.String())
	if lines[0].Type != "info" || !strings.Contains(lines[0].Payload, "Tun adapter listening at Meta") {
		t.Fatalf("parsed %#v", lines)
	}
}

func assertNoSecrets(t *testing.T, text string) {
	t.Helper()
	for _, secret := range []string{
		"super-secret-password",
		"11111111-2222-3333-4444-555555555555",
		"reality-public-key",
		"reality-private-key",
		"realityshort",
		"sub-token-value",
		"controller-token-value",
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret %s remained", secret)
		}
	}
}
