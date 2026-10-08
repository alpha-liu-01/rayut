package core

// Release is one mihomo build the app is allowed to install.
// The list is fixed in the program. There is no floating latest entry.
type Release struct {
	Tag           string
	URL           string
	SHA256        string
	License       string
	LicenseURL    string
	LicenseSHA256 string
	Source        string
}

// Catalog is the only set of cores the install path will fetch.
// Hashes are of the upstream gzip and the LICENSE file at that tag.
var Catalog = []Release{
	{
		Tag:           "v1.19.32",
		URL:           "https://github.com/MetaCubeX/mihomo/releases/download/v1.19.32/mihomo-linux-arm64-v1.19.32.gz",
		SHA256:        "9dd862e28b46ff7d775f169cceebc28deccaa0a9e804237d421cd2571e0caba0",
		License:       "GPL-3.0",
		LicenseURL:    "https://raw.githubusercontent.com/MetaCubeX/mihomo/v1.19.32/LICENSE",
		LicenseSHA256: "3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986",
		Source:        "https://github.com/MetaCubeX/mihomo/tree/v1.19.32",
	},
}

// Lookup returns a catalog entry by exact tag.
func Lookup(tag string) (Release, bool) {
	for _, rel := range Catalog {
		if rel.Tag == tag {
			return rel, true
		}
	}
	return Release{}, false
}
