// Package fabric talks to the Fabric meta API so we can install the Fabric loader
// into the official launcher WITHOUT requiring Java on the player's machine:
// we just write the loader's "profile" version JSON; the launcher downloads the
// libraries itself on first launch.
package fabric

import (
	"encoding/json"
	"fmt"

	"modpack-installer/internal/httpx"
)

const metaBase = "https://meta.fabricmc.net/v2"

type loaderEntry struct {
	Loader struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	} `json:"loader"`
}

// LatestStableLoader returns the newest stable loader version for a Minecraft version.
func LatestStableLoader(mcVersion string) (string, error) {
	var entries []loaderEntry
	url := fmt.Sprintf("%s/versions/loader/%s", metaBase, mcVersion)
	if err := httpx.JSON(url, &entries); err != nil {
		return "", fmt.Errorf("querying Fabric loaders for MC %s: %w", mcVersion, err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no Fabric loader available for Minecraft %s", mcVersion)
	}
	for _, e := range entries {
		if e.Loader.Stable {
			return e.Loader.Version, nil
		}
	}
	// Fall back to the newest (first) if none flagged stable.
	return entries[0].Loader.Version, nil
}

// Profile fetches the launcher-ready version JSON for (mcVersion, loader) and
// returns the raw bytes plus its "id" (the version folder name the launcher uses).
func Profile(mcVersion, loader string) (raw []byte, id string, err error) {
	url := fmt.Sprintf("%s/versions/loader/%s/%s/profile/json", metaBase, mcVersion, loader)
	raw, err = httpx.Bytes(url)
	if err != nil {
		return nil, "", fmt.Errorf("fetching Fabric profile JSON: %w", err)
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, "", fmt.Errorf("parsing Fabric profile JSON: %w", err)
	}
	if meta.ID == "" {
		return nil, "", fmt.Errorf("Fabric profile JSON has no \"id\"")
	}
	return raw, meta.ID, nil
}
