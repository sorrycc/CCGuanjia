// Package config is what the app, the ccgj command and the shell share:
// the default profile, each profile's launch options and the terminal to
// open, in config.json in ~/.config/ccguanjia (CCGUANJIA_CONFIG_DIR
// moves it), beside the shell file it writes.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Dir is the folder of the shared files.
func Dir(home string) string {
	if d := os.Getenv("CCGUANJIA_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(home, ".config", "ccguanjia")
}

// Terminals are the terminals Open in Terminal knows.
var Terminals = []string{"Terminal", "iTerm", "Ghostty"}

// Config is config.json.
type Config struct {
	// Default is the profile new shells start with, "" or "default" for
	// ~/.claude.
	Default string `json:"default,omitempty"`
	// Terminal is the app Open in Terminal uses.
	Terminal string `json:"terminal,omitempty"`
	// Language is the app's language, "" for the system's.
	Language string `json:"language,omitempty"`
	// BackupDir is where Cleanup puts its backups, "" for ~/Downloads.
	BackupDir string `json:"backupDir,omitempty"`
	// Profiles are the options of the profiles, by name.
	Profiles map[string]*ProfileOptions `json:"profiles,omitempty"`

	path string
}

// ProfileOptions are a profile's launch options.
type ProfileOptions struct {
	// SkipPermissions adds --dangerously-skip-permissions to its launcher.
	SkipPermissions bool `json:"skipPermissions,omitempty"`
}

// Load reads the config of dir; a missing or broken one reads as empty.
func Load(dir string) *Config {
	c := &Config{path: filepath.Join(dir, "config.json")}
	if data, err := os.ReadFile(c.path); err == nil {
		json.Unmarshal(data, c)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*ProfileOptions{}
	}
	return c
}

// Path is where the config is written.
func (c *Config) Path() string { return c.path }

// Options returns the options of a profile, which it adds if new.
func (c *Config) Options(name string) *ProfileOptions {
	o := c.Profiles[name]
	if o == nil {
		o = &ProfileOptions{}
		c.Profiles[name] = o
	}
	return o
}

// Known reports whether the config has options for a profile.
func (c *Config) Known(name string) bool { return c.Profiles[name] != nil }

// TerminalApp returns the terminal to open, Terminal by default.
func (c *Config) TerminalApp() string {
	if c.Terminal == "" {
		return Terminals[0]
	}
	return c.Terminal
}

// Save writes the config.
func (c *Config) Save() error {
	if c.path == "" {
		return errors.New("no path")
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
