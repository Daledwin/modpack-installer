// Package installer wires everything together: resolve Fabric, fetch the loader
// profile, then configure either the official launcher or Prism with the modpack's
// profiles (each gets its own game dir, modupdater.jar, and pre-filled servers).
package installer

import (
	"fmt"

	"modpack-installer/internal/config"
	"modpack-installer/internal/fabric"
	"modpack-installer/internal/httpx"
	"modpack-installer/internal/mcpaths"
)

type Options struct {
	Cfg      *config.Config
	Target   string   // "auto" | "official" | "prism"
	Profiles []string // profile keys to install; empty = all
	DryRun   bool
	Log      func(format string, a ...any)
}

type ProfileResult struct {
	Key        string
	Name       string
	GameDir    string
	ServersAdd int
}

type Result struct {
	Target        string
	MinecraftDir  string
	LoaderVersion string
	VersionID     string
	Profiles      []ProfileResult
}

func (o Options) logf(format string, a ...any) {
	if o.Log != nil {
		o.Log(format, a...)
	}
}

func (o Options) wantProfile(key string) bool {
	if len(o.Profiles) == 0 {
		return true
	}
	for _, k := range o.Profiles {
		if k == key {
			return true
		}
	}
	return false
}

// Install performs the full installation and returns a summary.
func Install(opts Options) (*Result, error) {
	cfg := opts.Cfg

	// 1. Resolve the Fabric loader version.
	loader := cfg.FabricLoaderVersion
	if loader == "" {
		opts.logf("Resolving latest stable Fabric loader for Minecraft %s…", cfg.MinecraftVersion)
		v, err := fabric.LatestStableLoader(cfg.MinecraftVersion)
		if err != nil {
			return nil, err
		}
		loader = v
	}
	opts.logf("Fabric loader: %s", loader)

	// 2. Fetch the launcher-ready Fabric profile JSON (no Java needed at install time).
	profileJSON, versionID, err := fabric.Profile(cfg.MinecraftVersion, loader)
	if err != nil {
		return nil, err
	}
	opts.logf("Fabric version id: %s", versionID)

	// 3. Choose the target launcher.
	target := opts.Target
	if target == "" || target == "auto" {
		if mcpaths.OfficialInstalled() {
			target = "official"
		} else if mcpaths.PrismInstalled() {
			target = "prism"
		} else {
			return nil, fmt.Errorf("no launcher found: neither the official Minecraft launcher (%s) nor Prism (%s) is installed", mcpaths.MinecraftDir(), mcpaths.PrismInstancesDir())
		}
	}
	opts.logf("Target launcher: %s", target)

	// 4. Download modupdater.jar once (skipped on dry-run).
	var modJar []byte
	if opts.DryRun {
		opts.logf("[dry-run] would download modupdater from %s", cfg.ModUpdaterJarURL)
	} else {
		opts.logf("Downloading modupdater from %s …", cfg.ModUpdaterJarURL)
		modJar, err = httpx.Bytes(cfg.ModUpdaterJarURL)
		if err != nil {
			return nil, fmt.Errorf("downloading modupdater jar: %w", err)
		}
		opts.logf("modupdater.jar: %d KB", len(modJar)/1024)
	}

	switch target {
	case "official":
		return installOfficial(opts, profileJSON, versionID, loader, modJar)
	case "prism":
		return installPrism(opts, versionID, loader, modJar)
	default:
		return nil, fmt.Errorf("unknown target launcher %q", target)
	}
}
