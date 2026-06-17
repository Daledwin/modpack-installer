// Package modpackindex fetches and parses the modpack repo's index.json
// (the same manifest modupdater consumes), so the installer can pre-download
// the client mod set instead of leaving it all to modupdater's first-launch sync.
package modpackindex

import (
	"fmt"

	"modpack-installer/internal/httpx"
)

type Entry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	File    string `json:"file"`
	Side    string `json:"side"` // server | client | both
	Sha256  string `json:"sha256"`
}

type Index struct {
	Mods []Entry `json:"mods"`
}

// Fetch downloads and parses index.json from the given URL.
func Fetch(indexURL string) (*Index, error) {
	var idx Index
	if err := httpx.JSON(indexURL, &idx); err != nil {
		return nil, fmt.Errorf("fetching modpack index %s: %w", indexURL, err)
	}
	return &idx, nil
}

// ClientMods returns the entries a client should install. Allowlist: only
// client/both (empty is treated as both, since the publisher always sets a side);
// "server" and any unknown value are excluded from the client.
func (i *Index) ClientMods() []Entry {
	var out []Entry
	for _, m := range i.Mods {
		switch m.Side {
		case "client", "both", "":
			out = append(out, m)
		}
	}
	return out
}
