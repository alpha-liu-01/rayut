package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GeoFile is one database placed beside the packaged mihomo binary.
// The upstream tag is "latest" and moves. SHA256 is the pin the build checks.
type GeoFile struct {
	Name   string
	URL    string
	SHA256 string
}

const (
	GeoLicense       = "GPL-3.0"
	GeoLicenseURL    = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/master/LICENSE"
	GeoLicenseSHA256 = "3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986"
	GeoSource        = "https://github.com/MetaCubeX/meta-rules-dat"
	// GeoPublished is the release time these hashes were taken from.
	GeoPublished = "2026-10-12T01:31:04Z"
)

// GeoFiles are copied into mihomo's home directory before it starts.
// Names are the ones mihomo v1.19.32 looks up: geoip.metadb, GeoSite.dat, GeoIP.dat.
var GeoFiles = []GeoFile{
	{
		Name:   "geoip.metadb",
		URL:    "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb",
		SHA256: "b8f447617c3f6d8ef0f7528c23dfba695bca16a31a396bc4c1fca49e9f038ae3",
	},
	{
		Name:   "GeoSite.dat",
		URL:    "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
		SHA256: "c951c1afc331ea82a46f568785d73d5bd1bcc99626739639ae9dfaa3f6aa6e71",
	},
	{
		Name:   "GeoIP.dat",
		URL:    "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat",
		SHA256: "116cc0f03d48991962f7f9cdbbcf53d45d89777f9c3cd1a663210853e5df6093",
	},
}

// installGeoFiles copies each packaged database into dest when the destination
// is missing or a different size. A file already the same size is left in place.
// A missing source is skipped so a development tree without the click payload
// can still start. The click build refuses to package without these files.
func installGeoFiles(srcDir, destDir string) error {
	for _, file := range GeoFiles {
		src := filepath.Join(srcDir, file.Name)
		info, err := os.Lstat(src)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		dest := filepath.Join(destDir, file.Name)
		if destInfo, err := os.Lstat(dest); err == nil && destInfo.Mode().IsRegular() && destInfo.Size() == info.Size() {
			continue
		}
		if err := copyRegularFile(src, dest); err != nil {
			return fmt.Errorf("geo data %s: %w", file.Name, err)
		}
	}
	return nil
}

func copyRegularFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
