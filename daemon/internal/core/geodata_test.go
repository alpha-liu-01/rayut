package core

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVendoredGeoFilesMatchCatalog(t *testing.T) {
	for _, file := range GeoFiles {
		data, err := os.ReadFile("../../../" + file.Path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("%s hash changed", file.Name)
		}
	}
}

func TestInstallGeoFilesCopiesMissingAndReplacesDifferentSize(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	name := GeoFiles[0].Name
	if err := os.WriteFile(filepath.Join(src, name), []byte("mmdb-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installGeoFiles(src, dest); err != nil {
		t.Fatal(err)
	}
	destPath := filepath.Join(dest, name)
	if got := readFile(t, destPath); got != "mmdb-bytes" {
		t.Fatalf("copied %q", got)
	}

	if err := os.WriteFile(destPath, []byte("MMDB-BYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installGeoFiles(src, dest); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, destPath); got != "MMDB-BYTES" {
		t.Fatalf("same-size file was replaced with %q", got)
	}

	if err := os.WriteFile(destPath, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installGeoFiles(src, dest); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, destPath); got != "mmdb-bytes" {
		t.Fatalf("different-size file stayed %q", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

func TestInstallGeoFilesSkipsMissingSource(t *testing.T) {
	dest := t.TempDir()
	if err := installGeoFiles(t.TempDir(), dest); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dest has %d entries", len(entries))
	}
}
