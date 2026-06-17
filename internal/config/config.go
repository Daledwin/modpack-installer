// Package config loads the modpack definition that drives the installer.
//
// Precedence: -config flag  >  modpack.config.json next to the executable  >  embedded default.
// This lets whoever ships the installer either rebuild with an edited embedded
// default, or just drop a modpack.config.json next to the binary.
package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed default.json
var embeddedDefault []byte

type Server struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Profile struct {
	Key    string `json:"key"`    // stable id, e.g. "prod" / "staging"
	Name   string `json:"name"`   // display name in the launcher
	Icon   string `json:"icon"`   // official-launcher icon name (e.g. "Grass")
	Branch string `json:"branch"` // informational (modupdater handles the sync)
}

// BaseMod is a mod the installer must place itself (bootstrap mods like Fabric API
// that modupdater needs to even start). Provide either a direct URL or a Modrinth
// project slug that is auto-resolved for the configured Minecraft version.
type BaseMod struct {
	Name     string `json:"name"`     // optional filename override
	URL      string `json:"url"`      // direct .jar URL (alternative to modrinth)
	Modrinth string `json:"modrinth"` // Modrinth project slug, e.g. "fabric-api"
}

type Config struct {
	ModpackName         string    `json:"modpackName"`
	MinecraftVersion    string    `json:"minecraftVersion"`
	FabricLoaderVersion string    `json:"fabricLoaderVersion"` // "" = latest stable
	ModUpdaterJarURL    string    `json:"modUpdaterJarUrl"`
	ModUpdaterJarName   string    `json:"modUpdaterJarName"`
	BaseMods            []BaseMod `json:"baseMods"` // bootstrap mods (e.g. Fabric API)
	Servers             []Server  `json:"servers"`
	Profiles            []Profile `json:"profiles"`
}

// Load resolves the config from the flag path, a sibling file, or the embedded default.
func Load(flagPath string) (*Config, string, error) {
	var data []byte
	var source string

	switch {
	case flagPath != "":
		b, err := os.ReadFile(flagPath)
		if err != nil {
			return nil, "", fmt.Errorf("reading --config %q: %w", flagPath, err)
		}
		data, source = b, flagPath
	default:
		if sib := siblingConfig(); sib != "" {
			b, err := os.ReadFile(sib)
			if err == nil {
				data, source = b, sib
			}
		}
		if data == nil {
			data, source = embeddedDefault, "(embedded default)"
		}
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, source, fmt.Errorf("parsing config %s: %w", source, err)
	}
	if err := c.validate(); err != nil {
		return nil, source, err
	}
	return &c, source, nil
}

func siblingConfig() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(exe), "modpack.config.json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.ModpackName) == "" {
		return fmt.Errorf("config: modpackName is required")
	}
	if strings.TrimSpace(c.MinecraftVersion) == "" {
		return fmt.Errorf("config: minecraftVersion is required")
	}
	if len(c.Profiles) == 0 {
		return fmt.Errorf("config: at least one profile is required")
	}
	if c.ModUpdaterJarName == "" {
		c.ModUpdaterJarName = "modupdater.jar"
	}
	return nil
}

// Placeholders reports config values that still contain the REPLACE-ME markers,
// so the installer can warn instead of silently shipping dummy data.
func (c *Config) Placeholders() []string {
	var out []string
	if strings.Contains(c.ModUpdaterJarURL, "REPLACE-ME") || c.ModUpdaterJarURL == "" {
		out = append(out, "modUpdaterJarUrl")
	}
	for _, s := range c.Servers {
		if strings.Contains(s.Address, "REPLACE-ME") {
			out = append(out, "server address: "+s.Name)
		}
	}
	for _, b := range c.BaseMods {
		if b.Modrinth == "" && strings.Contains(b.URL, "REPLACE-ME") {
			out = append(out, "baseMod url: "+b.Name)
		}
	}
	return out
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug returns a filesystem-safe identifier derived from the modpack name.
func (c *Config) Slug() string {
	s := strings.ToLower(strings.TrimSpace(c.ModpackName))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "modpack"
	}
	return s
}
