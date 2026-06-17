// Package modrinth resolves a project slug (e.g. "fabric-api") to a direct jar
// URL for a given Minecraft version + loader, so the installer never bundles or
// hard-codes a jar — it always fetches the right version at install time.
package modrinth

import (
	"fmt"
	"net/url"

	"modpack-installer/internal/httpx"
)

type file struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
}

type version struct {
	Name          string `json:"name"`
	VersionNumber string `json:"version_number"`
	Files         []file `json:"files"`
}

// Resolve returns the download URL and filename of the latest matching version.
func Resolve(slug, mcVersion, loader string) (jarURL, filename string, err error) {
	endpoint := fmt.Sprintf(
		"https://api.modrinth.com/v2/project/%s/version?game_versions=%s&loaders=%s",
		url.PathEscape(slug),
		url.QueryEscape(fmt.Sprintf("[%q]", mcVersion)),
		url.QueryEscape(fmt.Sprintf("[%q]", loader)),
	)
	var versions []version
	if err := httpx.JSON(endpoint, &versions); err != nil {
		return "", "", fmt.Errorf("querying Modrinth for %q: %w", slug, err)
	}
	if len(versions) == 0 {
		return "", "", fmt.Errorf("no %q version found for Minecraft %s (%s)", slug, mcVersion, loader)
	}
	// versions[0] is the newest. Prefer the primary file.
	v := versions[0]
	if len(v.Files) == 0 {
		return "", "", fmt.Errorf("Modrinth %q version %s has no files", slug, v.VersionNumber)
	}
	chosen := v.Files[0]
	for _, f := range v.Files {
		if f.Primary {
			chosen = f
			break
		}
	}
	return chosen.URL, chosen.Filename, nil
}
