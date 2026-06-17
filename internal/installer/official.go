package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"modpack-installer/internal/mcpaths"
)

func installOfficial(opts Options, profileJSON []byte, versionID, loader string, mods []Mod) (*Result, error) {
	cfg := opts.Cfg
	mcDir := mcpaths.MinecraftDir()
	slug := cfg.Slug()

	res := &Result{
		Target:        "official",
		MinecraftDir:  mcDir,
		LoaderVersion: loader,
		VersionID:     versionID,
	}

	// 1. Install the Fabric version (just the profile JSON; the launcher fetches libs).
	verDir := filepath.Join(mcDir, "versions", versionID)
	verFile := filepath.Join(verDir, versionID+".json")
	opts.logf("Fabric version -> %s", verFile)
	if !opts.DryRun {
		if err := os.MkdirAll(verDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(verFile, profileJSON, 0o644); err != nil {
			return nil, err
		}
	}

	// 2. Load launcher_profiles.json (preserve any existing content).
	lpPath := filepath.Join(mcDir, "launcher_profiles.json")
	root := map[string]any{}
	if b, err := os.ReadFile(lpPath); err == nil {
		// File exists: it MUST parse, otherwise overwriting it would wipe the
		// player's launcher profiles and accounts. Abort with a clear message.
		if uerr := json.Unmarshal(b, &root); uerr != nil {
			return nil, fmt.Errorf(
				"existing launcher_profiles.json is present but unparseable (%v); refusing to overwrite it — fix or remove %s, then retry",
				uerr, lpPath)
		}
	}
	profiles, _ := root["profiles"].(map[string]any)
	if profiles == nil {
		profiles = map[string]any{}
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// 3. One launcher profile per modpack profile.
	for _, p := range cfg.Profiles {
		if !opts.wantProfile(p.Key) {
			continue
		}
		gameDir := filepath.Join(mcDir, slug+"-"+p.Key)
		modsDir := filepath.Join(gameDir, "mods")
		opts.logf("Profile %q -> %s", p.Name, gameDir)

		if !opts.DryRun {
			if err := os.MkdirAll(modsDir, 0o755); err != nil {
				return nil, err
			}
			for _, m := range mods {
				jarPath := filepath.Join(modsDir, m.Name)
				if err := os.WriteFile(jarPath, m.Data, 0o644); err != nil {
					return nil, fmt.Errorf("writing %s: %w", jarPath, err)
				}
			}
		}

		added, err := mergeServers(filepath.Join(gameDir, "servers.dat"), cfg.Servers, opts.DryRun, opts.logf)
		if err != nil {
			return nil, err
		}

		icon := p.Icon
		if icon == "" {
			icon = "Furnace"
		}
		id := "modpack-" + slug + "-" + p.Key
		profiles[id] = map[string]any{
			"name":          p.Name,
			"type":          "custom",
			"created":       now,
			"lastUsed":      now,
			"lastVersionId": versionID,
			"gameDir":       gameDir,
			"icon":          icon,
		}

		res.Profiles = append(res.Profiles, ProfileResult{
			Key: p.Key, Name: p.Name, GameDir: gameDir, Mods: len(mods), ServersAdd: added,
		})
	}

	// 4. Write launcher_profiles.json back.
	root["profiles"] = profiles
	if _, ok := root["version"]; !ok {
		root["version"] = float64(3)
	}
	if !opts.DryRun {
		out, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(mcDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(lpPath, out, 0o644); err != nil {
			return nil, err
		}
	}
	opts.logf("launcher_profiles.json updated (%d profile(s))", len(res.Profiles))

	return res, nil
}
