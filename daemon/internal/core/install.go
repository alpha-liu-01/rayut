package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
	"github.com/alpha-liu-01/rayut/daemon/internal/route"
)

const (
	errDisconnect = codeError("disconnect first")
	errUnknown    = codeError("unknown release")
	errHash       = codeError("hash mismatch")
	errLicense    = codeError("license mismatch")
	errDownload   = codeError("download failed")
	errInvalid    = codeError("invalid file")
)

type codeError string

func (e codeError) Error() string { return string(e) }

var httpClient = &http.Client{
	Timeout: 3 * time.Minute,
	Transport: &http.Transport{
		Proxy: nil,
	},
}

// coreHeld reports whether a core process or its TUN interface is still up.
var coreHeld = func() bool {
	if _, ok := Alive(); ok {
		return true
	}
	present, err := route.TunPresent()
	return err == nil && present
}

// Install downloads one catalog tag, checks the gzip and the license, then
// replaces the data-directory core. A failed check leaves the previous file.
func Install(ctx context.Context, tag string) error {
	rel, ok := Lookup(tag)
	if !ok || tag == "" || tag == "latest" {
		return errUnknown
	}
	if coreHeld() {
		return errDisconnect
	}
	return installRelease(ctx, rel)
}

func installRelease(ctx context.Context, rel Release) error {
	if coreHeld() {
		return errDisconnect
	}
	if rel.URL == "" || rel.SHA256 == "" || rel.LicenseURL == "" || rel.LicenseSHA256 == "" {
		return errUnknown
	}
	dir := filepath.Dir(paths.CoreFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errDownload
	}
	ClearInstallTemps()
	gzPath := filepath.Join(dir, "download.gz")
	defer os.Remove(gzPath)
	if err := download(ctx, rel.URL, gzPath, 32<<20); err != nil {
		return errDownload
	}
	if !fileHash(gzPath, rel.SHA256) {
		return errHash
	}
	licPath := filepath.Join(dir, "LICENSE.download")
	defer os.Remove(licPath)
	if err := download(ctx, rel.LicenseURL, licPath, 256<<10); err != nil {
		return errDownload
	}
	if !fileHash(licPath, rel.LicenseSHA256) {
		return errLicense
	}
	license, err := os.ReadFile(licPath)
	if err != nil || !bytes.Contains(license, []byte("GNU GENERAL PUBLIC LICENSE")) {
		return errLicense
	}
	if coreHeld() {
		return errDisconnect
	}
	partial := paths.CoreFile + ".partial"
	os.Remove(partial)
	defer os.Remove(partial)
	if err := gunzipELF(gzPath, partial); err != nil {
		return errInvalid
	}
	if err := os.Chmod(partial, 0o755); err != nil {
		return errInvalid
	}
	if err := os.Rename(partial, paths.CoreFile); err != nil {
		return errInvalid
	}
	if err := os.WriteFile(paths.CoreTag, []byte(rel.Tag+"\n"), 0o644); err != nil {
		return errInvalid
	}
	return nil
}

func download(ctx context.Context, rawURL, dest string, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "rayut")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("status")
	}
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	n, err := io.Copy(file, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return errors.New("too large")
	}
	return nil
}

func fileHash(path, want string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return false
	}
	got := hex.EncodeToString(sum.Sum(nil))
	return strings.EqualFold(got, want)
}

func gunzipELF(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	reader, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer reader.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	header := make([]byte, 20)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if !aarch64ELF(header) {
		return errors.New("elf")
	}
	if _, err := out.Write(header); err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(reader, 64<<20))
	if err != nil {
		return err
	}
	if n >= 64<<20 {
		return errors.New("too large")
	}
	return nil
}

func aarch64ELF(header []byte) bool {
	if len(header) < 20 {
		return false
	}
	if !bytes.Equal(header[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return false
	}
	if header[4] != 2 || header[5] != 1 {
		return false
	}
	return binary.LittleEndian.Uint16(header[18:20]) == 183
}

// ClearInstallTemps removes the download, the license copy, and a half-written
// binary. The installed core and its tag are left in place.
func ClearInstallTemps() {
	dir := filepath.Dir(paths.CoreFile)
	os.Remove(filepath.Join(dir, "download.gz"))
	os.Remove(filepath.Join(dir, "LICENSE.download"))
	os.Remove(paths.CoreFile + ".partial")
}

// UsingDataCopy reports whether the next start will use the data-directory core.
func UsingDataCopy() bool {
	info, err := os.Lstat(paths.CoreFile)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

// InstalledTag is the catalog tag recorded beside a data-directory core.
func InstalledTag() string {
	data, err := os.ReadFile(paths.CoreTag)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// CatalogView is the list the interface shows. It has no download URL.
func CatalogView() map[string]any {
	tag := ""
	place := "app"
	if UsingDataCopy() {
		place = "data"
		tag = InstalledTag()
	}
	releases := make([]map[string]any, 0, len(Catalog))
	for _, rel := range Catalog {
		releases = append(releases, map[string]any{
			"tag":       rel.Tag,
			"license":   rel.License,
			"source":    rel.Source,
			"installed": place == "data" && tag == rel.Tag,
		})
	}
	return map[string]any{
		"place":    place,
		"tag":      tag,
		"releases": releases,
	}
}
