package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
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
	if err := Install(context.Background(), "latest"); err == nil || err.Error() != "unknown release" {
		t.Fatal(err)
	}
}

func TestWrongHashKeepsThePreviousCore(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	previous := []byte("previous-core")
	if err := os.WriteFile(paths.CoreFile, previous, 0o755); err != nil {
		t.Fatal(err)
	}
	elf := fakeELF()
	body := gzipBytes(t, elf)
	license := []byte("GNU GENERAL PUBLIC LICENSE\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/license" {
			_, _ = w.Write(license)
			return
		}
		_, _ = w.Write([]byte("not-the-binary"))
	}))
	defer srv.Close()
	useClient(t, srv.Client())
	rel := Release{
		Tag:           "v9.9.9",
		URL:           srv.URL + "/core.gz",
		SHA256:        hex.EncodeToString(sha256Sum(body)),
		License:       "GPL-3.0",
		LicenseURL:    srv.URL + "/license",
		LicenseSHA256: hex.EncodeToString(sha256Sum(license)),
		Source:        "https://example.test/source",
	}
	if err := installRelease(context.Background(), rel); err == nil || err.Error() != "hash mismatch" {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.CoreFile)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatalf("previous core changed: %q %v", got, err)
	}
	if _, err := os.Stat(paths.CoreFile + ".partial"); !os.IsNotExist(err) {
		t.Fatal("partial left behind")
	}
}

func TestWrongLicenseKeepsThePreviousCore(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	previous := []byte("previous-core")
	if err := os.WriteFile(paths.CoreFile, previous, 0o755); err != nil {
		t.Fatal(err)
	}
	body := gzipBytes(t, fakeELF())
	license := []byte("not a license")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/license" {
			_, _ = w.Write(license)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	useClient(t, srv.Client())
	rel := Release{
		Tag:           "v9.9.9",
		URL:           srv.URL + "/core.gz",
		SHA256:        hex.EncodeToString(sha256Sum(body)),
		LicenseURL:    srv.URL + "/license",
		LicenseSHA256: hex.EncodeToString(sha256Sum([]byte("GNU GENERAL PUBLIC LICENSE\n"))),
		Source:        "https://example.test/source",
	}
	if err := installRelease(context.Background(), rel); err == nil || err.Error() != "license mismatch" {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.CoreFile)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatalf("previous core changed: %q %v", got, err)
	}
}

func TestMatchingHashReplacesTheDataCopy(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	if err := os.WriteFile(paths.CoreFile, []byte("previous-core"), 0o755); err != nil {
		t.Fatal(err)
	}
	elf := fakeELF()
	body := gzipBytes(t, elf)
	license := []byte("prefix GNU GENERAL PUBLIC LICENSE suffix\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/license" {
			_, _ = w.Write(license)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	useClient(t, srv.Client())
	rel := Release{
		Tag:           "v1.2.3",
		URL:           srv.URL + "/core.gz",
		SHA256:        hex.EncodeToString(sha256Sum(body)),
		License:       "GPL-3.0",
		LicenseURL:    srv.URL + "/license",
		LicenseSHA256: hex.EncodeToString(sha256Sum(license)),
		Source:        "https://example.test/source",
	}
	if err := installRelease(context.Background(), rel); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.CoreFile)
	if err != nil || !bytes.Equal(got, elf) {
		t.Fatalf("installed %d bytes", len(got))
	}
	if InstalledTag() != "v1.2.3" || !UsingDataCopy() {
		t.Fatalf("tag %q data %v", InstalledTag(), UsingDataCopy())
	}
	if Executable() != paths.CoreFile {
		t.Fatal(Executable())
	}
}

func TestHeldCoreIsNotReplaced(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	previous := []byte("previous-core")
	if err := os.WriteFile(paths.CoreFile, previous, 0o755); err != nil {
		t.Fatal(err)
	}
	old := coreHeld
	coreHeld = func() bool { return true }
	t.Cleanup(func() { coreHeld = old })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("download started while held")
	}))
	defer srv.Close()
	useClient(t, srv.Client())
	rel := Release{
		Tag:           "v1.2.3",
		URL:           srv.URL + "/core.gz",
		SHA256:        "aa",
		LicenseURL:    srv.URL + "/license",
		LicenseSHA256: "bb",
	}
	if err := installRelease(context.Background(), rel); err == nil || err.Error() != "disconnect first" {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.CoreFile)
	if err != nil || !bytes.Equal(got, previous) {
		t.Fatal("replaced while held")
	}
}

func TestClearInstallTempsKeepsTheInstalledCore(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	kept := []byte("kept-core")
	if err := os.WriteFile(paths.CoreFile, kept, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.CoreTag, []byte("v1.19.32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(paths.CoreFile)
	for _, name := range []string{"download.gz", "LICENSE.download"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("temp"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(paths.CoreFile+".partial", []byte("half"), 0o755); err != nil {
		t.Fatal(err)
	}
	ClearInstallTemps()
	got, err := os.ReadFile(paths.CoreFile)
	if err != nil || !bytes.Equal(got, kept) {
		t.Fatal("core removed")
	}
	if InstalledTag() != "v1.19.32" {
		t.Fatal(InstalledTag())
	}
	for _, name := range []string{"download.gz", "LICENSE.download", "mihomo.partial"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatal(name)
		}
	}
}

func TestSymlinkIsNotTheNextCore(t *testing.T) {
	root := t.TempDir()
	useCoreDir(t, root)
	other := filepath.Join(root, "other")
	if err := os.WriteFile(other, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, paths.CoreFile); err != nil {
		t.Fatal(err)
	}
	if UsingDataCopy() {
		t.Fatal("symlink selected")
	}
	if Executable() != paths.Mihomo {
		t.Fatal(Executable())
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

func useClient(t *testing.T, client *http.Client) {
	t.Helper()
	old := httpClient
	httpClient = client
	t.Cleanup(func() { httpClient = old })
}

func fakeELF() []byte {
	buf := make([]byte, 64)
	copy(buf, []byte{0x7f, 'E', 'L', 'F', 2, 1})
	binary.LittleEndian.PutUint16(buf[18:], 183)
	return buf
}

func gzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
