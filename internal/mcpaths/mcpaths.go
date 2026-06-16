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

// OfficialInstalled reports whether the official launcher looks present
// (its launcher_profiles.json or the .minecraft dir exists).
func OfficialInstalled() bool {
	dir := MinecraftDir()
	if fi, err := os.Stat(filepath.Join(dir, "launcher_profiles.json")); err == nil && !fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return true
	}
	return false
}

// PrismInstancesDir returns Prism Launcher's instances directory (best-effort).
func PrismInstancesDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "PrismLauncher", "instances")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "PrismLauncher", "instances")
	default:
		return filepath.Join(home, ".local", "share", "PrismLauncher", "instances")
	}
}

// PrismInstalled reports whether Prism's data directory exists.
func PrismInstalled() bool {
	dir := filepath.Dir(PrismInstancesDir()) // the PrismLauncher data dir
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return true
	}
	return false
}
