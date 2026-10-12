package core

import (
	"os"
	"path/filepath"

	"github.com/alpha-liu-01/rayut/daemon/internal/paths"
)

// ClearInstallTemps removes leftovers from the old in-app core download,
// including a core that was stored in the data directory.
func ClearInstallTemps() {
	dir := filepath.Dir(paths.CoreFile)
	os.Remove(filepath.Join(dir, "download.gz"))
	os.Remove(filepath.Join(dir, "LICENSE.download"))
	os.Remove(paths.CoreFile + ".partial")
	os.Remove(paths.CoreFile)
	os.Remove(paths.CoreTag)
}

// CatalogView is the list the interface shows. It has no download URL.
func CatalogView() map[string]any {
	tag := ""
	if len(Catalog) > 0 {
		tag = Catalog[0].Tag
	}
	releases := make([]map[string]any, 0, len(Catalog))
	for _, rel := range Catalog {
		releases = append(releases, map[string]any{
			"tag":       rel.Tag,
			"license":   rel.License,
			"source":    rel.Source,
			"installed": false,
		})
	}
	return map[string]any{
		"place":    "app",
		"tag":      tag,
		"releases": releases,
	}
}
