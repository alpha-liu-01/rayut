package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

func TestCatalogHasNoLatest(t *testing.T) {
	if len(Catalog) == 0 {
		t.Fatal("empty catalog")
	}
	for _, rel := range Catalog {
		if rel.Tag == "" || rel.Tag == "latest" || rel.URL == "" || rel.Source == "" || rel.License == "" || rel.LicenseURL == "" {
			t.Fatalf("incomplete entry %q", rel.Tag)
		}
		if len(rel.SHA256) != 64 || len(rel.LicenseSHA256) != 64 {
			t.Fatalf("hash length %s", rel.Tag)
		}
		if _, ok := Lookup(rel.Tag); !ok {
			t.Fatal(rel.Tag)
		}
	}
	if _, ok := Lookup("latest"); ok {
		t.Fatal("latest is listed")
	}
	view := CatalogView()
	if view["place"] != "app" || view["tag"] != "v1.19.32" {
		t.Fatalf("%v", view)
	}
}

func TestExecutableIgnoresDataDirectory(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	old := paths.Mihomo
	paths.Mihomo = filepath.Join(root, "packaged")
	t.Cleanup(func() { paths.Mihomo = old })
	if err := os.WriteFile(paths.CoreFile, []byte("downloaded"), 0o755); err != nil {
		t.Fatal(err)
	}
	if Executable() != paths.Mihomo {
		t.Fatal(Executable())
	}
}

func TestClearInstallTempsRemovesDownloadedCore(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	dir := filepath.Dir(paths.CoreFile)
	for _, name := range []string{"download.gz", "LICENSE.download"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(paths.CoreFile, []byte("downloaded"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CoreFile+".partial", []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CoreTag, []byte("v1.19.32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ClearInstallTemps()
	for _, name := range []string{paths.CoreFile, paths.CoreFile + ".partial", paths.CoreTag, filepath.Join(dir, "download.gz"), filepath.Join(dir, "LICENSE.download")} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatal(name)
		}
	}
}

func useCoreDir(t *testing.T, root string) {
	t.Helper()
	oldFile, oldTag := paths.CoreFile, paths.CoreTag
	paths.CoreFile = filepath.Join(root, "core", "mihomo")
	paths.CoreTag = filepath.Join(root, "core", "tag")
	if err := os.MkdirAll(filepath.Dir(paths.CoreFile), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		paths.CoreFile = oldFile
		paths.CoreTag = oldTag
	})
}
