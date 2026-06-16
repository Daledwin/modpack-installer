// modpack-installer: sets up the official Minecraft launcher (or Prism) with
// Fabric, the modupdater mod, and the modpack's servers pre-configured.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"modpack-installer/internal/config"
	"modpack-installer/internal/installer"
	"modpack-installer/internal/mcpaths"
)

func main() {
	var cfgPath, target, profilesArg string
	var dryRun, yes bool
	flag.StringVar(&cfgPath, "config", "", "path to modpack.config.json (default: sibling file or embedded)")
	flag.StringVar(&target, "launcher", "auto", "which launcher: auto | official | prism")
	flag.StringVar(&profilesArg, "profiles", "", "comma-separated profile keys to install (default: all)")
	flag.BoolVar(&dryRun, "dry-run", false, "preview the actions without writing anything")
	flag.BoolVar(&yes, "y", false, "skip the confirmation prompt")
	flag.Parse()

	cfg, source, err := config.Load(cfgPath)
	if err != nil {
		fatal(err)
	}

	fmt.Println()
	fmt.Printf("  ┌─ %s installer\n", cfg.ModpackName)
	fmt.Printf("  │  minecraft : %s (Fabric)\n", cfg.MinecraftVersion)
	fmt.Printf("  │  config    : %s\n", source)

	// Detect launcher
	detected := "none"
	if mcpaths.OfficialInstalled() {
		detected = "official (" + mcpaths.MinecraftDir() + ")"
	} else if mcpaths.PrismInstalled() {
		detected = "prism (" + mcpaths.PrismInstancesDir() + ")"
	}
	fmt.Printf("  │  launcher  : %s\n", detected)
	fmt.Printf("  │  profiles  : %s\n", profileNames(cfg))
	fmt.Printf("  │  servers   : %s\n", serverNames(cfg))
	fmt.Printf("  └─\n\n")

	// Placeholder guard
	ph := cfg.Placeholders()
	if len(ph) > 0 {
		fmt.Printf("  ⚠ config still has placeholders: %s\n", strings.Join(ph, ", "))
		for _, p := range ph {
			if strings.HasPrefix(p, "modUpdaterJarUrl") && !dryRun {
				fatal(fmt.Errorf("modUpdaterJarUrl is not set — edit modpack.config.json before a real run (or use --dry-run)"))
			}
		}
		fmt.Println()
	}

	if detected == "none" && (target == "auto" || target == "") {
		fatal(fmt.Errorf("no launcher found — install the official Minecraft launcher or Prism first"))
	}

	if !yes && !dryRun {
		fmt.Print("  Proceed with installation? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "y" && ans != "yes" && ans != "o" && ans != "oui" {
			fmt.Println("  Aborted.")
			pauseOnWindows()
			return
		}
	}
	fmt.Println()

	var keys []string
	for _, k := range strings.Split(profilesArg, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}

	res, err := installer.Install(installer.Options{
		Cfg:      cfg,
		Target:   target,
		Profiles: keys,
		DryRun:   dryRun,
		Log:      func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) },
	})
	if err != nil {
		fatal(err)
	}

	fmt.Println()
	if dryRun {
		fmt.Println("  ✓ Dry-run complete — nothing was written.")
	} else {
		fmt.Printf("  ✓ Installed via the %s launcher.\n", res.Target)
	}
	for _, p := range res.Profiles {
		fmt.Printf("    • %-22s  %s  (+%d server(s))\n", p.Name, p.GameDir, p.ServersAdd)
	}
	fmt.Println()
	fmt.Printf("  Next: open your launcher, pick the profile \"%s\", and play.\n", firstProfileName(res, cfg))
	fmt.Println("  modupdater will sync the rest of the mods on first launch.")
	pauseOnWindows()
}

func profileNames(c *config.Config) string {
	var n []string
	for _, p := range c.Profiles {
		n = append(n, p.Name)
	}
	return strings.Join(n, ", ")
}

func serverNames(c *config.Config) string {
	var n []string
	for _, s := range c.Servers {
		n = append(n, s.Name)
	}
	if len(n) == 0 {
		return "(none)"
	}
	return strings.Join(n, ", ")
}

func firstProfileName(res *installer.Result, c *config.Config) string {
	if len(res.Profiles) > 0 {
		return res.Profiles[0].Name
	}
	if len(c.Profiles) > 0 {
		return c.Profiles[0].Name
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "\n  ✗ %v\n", err)
	pauseOnWindows()
	os.Exit(1)
}

// pauseOnWindows keeps a double-clicked console window open so the user can read output.
func pauseOnWindows() {
	if runtime.GOOS == "windows" {
		fmt.Print("\n  Press Enter to close…")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
}
