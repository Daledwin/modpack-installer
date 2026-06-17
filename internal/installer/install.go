// Package installer wires everything together: resolve Fabric, fetch the loader
// profile, then configure either the official launcher or Prism with the modpack's
// profiles (each gets its own game dir, modupdater.jar, and pre-filled servers).
package installer

import (
	"fmt"
	"net/url"
	"strings"

	"modpack-installer/internal/config"
	"modpack-installer/internal/fabric"
	"modpack-installer/internal/httpx"
	"modpack-installer/internal/mcpaths"
	"modpack-installer/internal/modrinth"
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
	Mods       int
	ServersAdd int
}

// Mod is a jar the installer places into each profile's mods/ folder.
type Mod struct {
	Name string
	URL  string
	Data []byte // nil on dry-run
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

	// 4. Resolve the jars the installer must place itself: modupdater plus any
	//    bootstrap base mods (e.g. Fabric API, auto-resolved from Modrinth).
	//    Nothing is bundled in the binary — everything is fetched at install time.
	mods, err := resolveMods(opts, loader)
	if err != nil {
		return nil, err
	}

	switch target {
	case "official":
		return installOfficial(opts, profileJSON, versionID, loader, mods)
	case "prism":
		return installPrism(opts, versionID, loader, mods)
	default:
		return nil, fmt.Errorf("unknown target launcher %q", target)
	}
}

func resolveMods(opts Options, loader string) ([]Mod, error) {
	cfg := opts.Cfg
	specs := []Mod{{Name: cfg.ModUpdaterJarName, URL: cfg.ModUpdaterJarURL}}

	for _, bm := range cfg.BaseMods {
		var u, name string
		switch {
		case bm.Modrinth != "":
			ru, fn, err := modrinth.Resolve(bm.Modrinth, cfg.MinecraftVersion, "fabric")
			if err != nil {
				return nil, err
			}
			u, name = ru, fn
			opts.logf("Resolved %s -> %s", bm.Modrinth, fn)
		case bm.URL != "":
			u, name = bm.URL, lastSegment(bm.URL)
		default:
			return nil, fmt.Errorf("baseMod entry must set either \"modrinth\" or \"url\"")
		}
		if bm.Name != "" {
			name = bm.Name
		}
		specs = append(specs, Mod{Name: sanitizeJar(name), URL: u})
	}

	for i := range specs {
		if opts.DryRun {
			opts.logf("[dry-run] would download %s", specs[i].Name)
			continue
		}
		opts.logf("Downloading %s …", specs[i].Name)
		b, err := httpx.Bytes(specs[i].URL)
		if err != nil {
			return nil, fmt.Errorf("downloading %s: %w", specs[i].Name, err)
		}
		specs[i].Data = b
		opts.logf("  %s: %d KB", specs[i].Name, len(b)/1024)
	}
	return specs, nil
}

func lastSegment(u string) string {
	s := u
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	if dec, err := url.PathUnescape(s); err == nil {
		s = dec
	}
	if s == "" {
		return "mod.jar"
	}
	return s
}

func sanitizeJar(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "mod.jar"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".jar") {
		name += ".jar"
	}
	return name
}
