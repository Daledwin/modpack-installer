package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"modpack-installer/internal/mcpaths"
)

// installPrism creates one Prism instance per modpack profile. Prism resolves
// the Fabric loader (and intermediary mappings) itself from mmc-pack.json, so we
// only declare the components — no Fabric profile JSON needed here.
// instancesDir, when non-empty, overrides Prism's detected instances directory
// (used for a freshly auto-installed portable Prism).
func installPrism(opts Options, versionID, loader string, mods []Mod, instancesDir string) (*Result, error) {
	cfg := opts.Cfg
	if instancesDir == "" {
		instancesDir = mcpaths.PrismInstancesDir()
	}
	slug := cfg.Slug()

	res := &Result{
		Target:        "prism",
		MinecraftDir:  instancesDir,
		LoaderVersion: loader,
		VersionID:     versionID,
	}

	for _, p := range cfg.Profiles {
		if !opts.wantProfile(p.Key) {
			continue
		}
		instDir := filepath.Join(instancesDir, slug+"-"+p.Key)
		dotMC := filepath.Join(instDir, ".minecraft")
		modsDir := filepath.Join(dotMC, "mods")
		opts.logf("Prism instance %q -> %s", p.Name, instDir)

		if !opts.DryRun {
			if err := os.MkdirAll(modsDir, 0o755); err != nil {
				return nil, err
			}
			// mmc-pack.json — component list Prism understands.
			pack := map[string]any{
				"formatVersion": 1,
				"components": []any{
					map[string]any{"uid": "net.minecraft", "version": cfg.MinecraftVersion, "important": true},
					map[string]any{"uid": "net.fabricmc.fabric-loader", "version": loader},
				},
			}
			packBytes, _ := json.MarshalIndent(pack, "", "  ")
			if err := os.WriteFile(filepath.Join(instDir, "mmc-pack.json"), packBytes, 0o644); err != nil {
				return nil, err
			}
			// instance.cfg — display name + type.
			cfgIni := fmt.Sprintf("[General]\nConfigVersion=1.2\nInstanceType=OneSix\nname=%s\niconKey=default\n", p.Name)
			if err := os.WriteFile(filepath.Join(instDir, "instance.cfg"), []byte(cfgIni), 0o644); err != nil {
				return nil, err
			}
			// modupdater + base mods (e.g. Fabric API)
			for _, m := range mods {
				if err := os.WriteFile(filepath.Join(modsDir, m.Name), m.Data, 0o644); err != nil {
					return nil, err
				}
			}
		}

		added, err := mergeServers(filepath.Join(dotMC, "servers.dat"), cfg.Servers, opts.DryRun, opts.logf)
		if err != nil {
			return nil, err
		}

		res.Profiles = append(res.Profiles, ProfileResult{
			Key: p.Key, Name: p.Name, GameDir: dotMC, Mods: len(mods), ServersAdd: added,
		})
	}
	return res, nil
}
