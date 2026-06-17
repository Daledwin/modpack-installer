// Package mcpaths resolves per-OS locations for the official Minecraft launcher
// and for Prism Launcher.
package mcpaths

import (
	"os"
	"path/filepath"
	"runtime"
)

// MinecraftDir returns the official launcher's .minecraft directory for this OS.
func MinecraftDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, ".minecraft")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "minecraft")
	default: // linux & others
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".minecraft")
	}
}

// OfficialInstalled reports whether the official launcher is really present.
// We require a launcher_profiles.json or a versions/ directory — a bare, empty
// ~/.minecraft (a common leftover from other launchers) must NOT shadow Prism.
func OfficialInstalled() bool {
	dir := MinecraftDir()
	if fi, err := os.Stat(filepath.Join(dir, "launcher_profiles.json")); err == nil && !fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(filepath.Join(dir, "versions")); err == nil && fi.IsDir() {
		return true
	}
	return false
}

// prismDataCandidates lists the possible Prism data dirs for this OS, in priority order.
func prismDataCandidates() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return []string{filepath.Join(appData, "PrismLauncher")}
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "PrismLauncher")}
	default: // linux: native (XDG), default, and Flatpak (the most common Linux Prism)
		var c []string
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			c = append(c, filepath.Join(xdg, "PrismLauncher"))
		}
		c = append(c,
			filepath.Join(home, ".local", "share", "PrismLauncher"),
			filepath.Join(home, ".var", "app", "org.prismlauncher.PrismLauncher", "data", "PrismLauncher"),
		)
		return c
	}
}

// prismDataDir returns the first existing Prism data dir, or "" if none.
func prismDataDir() string {
	for _, d := range prismDataCandidates() {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return ""
}

// PrismInstalled reports whether a Prism data directory exists (native or Flatpak).
func PrismInstalled() bool {
	return prismDataDir() != ""
}

// PrismInstancesDir returns Prism's instances directory (detected, else the default).
func PrismInstancesDir() string {
	d := prismDataDir()
	if d == "" {
		d = prismDataCandidates()[0] // default install target
	}
	return filepath.Join(d, "instances")
}
