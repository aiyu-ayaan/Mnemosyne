// Package config resolves where Mnemosyne keeps its memories.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvRoot overrides the memory root for a single run.
const EnvRoot = "MNEMOSYNE_ROOT"

const appDir = "mnemosyne"

// Config is the on-disk settings file. It lives outside the memory root, since
// it is what tells Mnemosyne where that root is.
type Config struct {
	Root string `json:"root,omitempty"`
}

// Source records where the memory root came from, so `doctor` can explain a
// surprising answer instead of just stating it.
type Source string

const (
	SourceFlag    Source = "--root flag"
	SourceEnv     Source = "MNEMOSYNE_ROOT"
	SourceFile    Source = "config file"
	SourceDefault Source = "default"
)

// Path is the settings file location: <user config dir>/mnemosyne/config.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(dir, appDir, "config.json"), nil
}

// DefaultRoot is where memories go when nothing says otherwise:
// <user config dir>/mnemosyne/memories.
//
// Keeping the settings file and the default root under one directory means the
// same layout on every platform, rather than three special cases that are easy
// to confuse with each other.
func DefaultRoot() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(dir, appDir, "memories"), nil
}

// ResolveRoot picks the memory root, most specific wins: the --root flag, then
// MNEMOSYNE_ROOT, then the config file, then the default.
func ResolveRoot(flagRoot string) (string, Source, error) {
	if v := strings.TrimSpace(flagRoot); v != "" {
		abs, err := filepath.Abs(v)
		return abs, SourceFlag, err
	}
	if v := strings.TrimSpace(os.Getenv(EnvRoot)); v != "" {
		abs, err := filepath.Abs(v)
		return abs, SourceEnv, err
	}

	cfg, err := Load()
	if err != nil {
		return "", "", err
	}
	if v := strings.TrimSpace(cfg.Root); v != "" {
		abs, err := filepath.Abs(v)
		return abs, SourceFile, err
	}

	root, err := DefaultRoot()
	return root, SourceDefault, err
}

// Load reads the settings file. A missing file is not an error: it means the
// user has not chosen anything yet, which is the normal first run.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the settings file, creating its directory if needed.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
