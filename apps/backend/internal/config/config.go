// Package config resolves where Mnemosyne keeps its settings and its memories.
//
// Two layouts exist. Installed, everything lives under the user config
// directory. Portable, everything lives beside the binary and nothing outside
// that folder is ever written.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aiyu-ayaan/mnemosyne/internal/embed"
)

// EnvRoot overrides the memory root for a single run.
const EnvRoot = "MNEMOSYNE_ROOT"

// PortableMarker next to the binary switches on portable mode. A marker file
// rather than a flag is what lets an unzipped folder be portable without the
// user having to remember anything.
const PortableMarker = "mnemosyne.portable"

// EnvDev switches on development mode. A checkout must never read, write, or
// index the memories a person actually relies on, so dev mode gets its own
// settings file, memory root, runtime directory and channel, and refuses to
// install itself or register anything at logon.
const EnvDev = "MNEMOSYNE_DEV"

const appDir = "mnemosyne"

// devAppDir keeps every dev path a sibling of the real one rather than a
// subdirectory of it, so deleting it cannot take real memories with it.
const devAppDir = "mnemosyne-dev"

// Config is the settings file.
type Config struct {
	Root  string         `json:"root,omitempty"`
	Embed embed.Settings `json:"embed,omitempty"`
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

// Locations is the resolved layout for this run.
type Locations struct {
	// Portable reports whether Mnemosyne is confined to its own directory.
	Portable bool

	// Dev reports whether this is a development run, kept apart from the
	// installed copy's data and forbidden from installing itself.
	Dev bool

	// BinDir is the directory holding the running binary.
	BinDir string

	// ConfigPath is the settings file.
	ConfigPath string

	// DefaultRoot is the memory root used when nothing else selects one.
	DefaultRoot string

	// RuntimeDir holds files that describe the running daemon — the token, the
	// unix socket, the pid. It is not the memory root: nothing in here survives
	// a reboot, and deleting it costs a daemon restart and nothing else.
	RuntimeDir string
}

// Detect works out the layout. forcePortable corresponds to --portable and
// turns portable mode on even without the marker file.
func Detect(forcePortable bool) (Locations, error) {
	binDir, err := binaryDir()
	if err != nil {
		return Locations{}, err
	}

	dev := os.Getenv(EnvDev) == "1"
	dir := appDir
	if dev {
		dir = devAppDir
	}

	portable := forcePortable
	if !portable {
		if _, err := os.Stat(filepath.Join(binDir, PortableMarker)); err == nil {
			portable = true
		}
	}

	if portable {
		return Locations{
			Portable:    true,
			Dev:         dev,
			BinDir:      binDir,
			ConfigPath:  filepath.Join(binDir, "config.json"),
			DefaultRoot: filepath.Join(binDir, "memories"),
			RuntimeDir:  filepath.Join(binDir, "run"),
		}, nil
	}

	// The settings file cannot live inside the memory root, because it is what
	// says where that root is. Both sit under the user config directory, the
	// same shape on every platform.
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Locations{}, fmt.Errorf("locate the user config directory: %w", err)
	}
	runtime, err := runtimeDir(dir)
	if err != nil {
		return Locations{}, err
	}
	return Locations{
		Dev:         dev,
		BinDir:      binDir,
		ConfigPath:  filepath.Join(configDir, dir, "config.json"),
		DefaultRoot: filepath.Join(configDir, dir, "memories"),
		RuntimeDir:  runtime,
	}, nil
}

// Isolated reports whether this copy must not share a channel with the
// installed daemon — true for both portable and development runs.
func (l Locations) Isolated() bool { return l.Portable || l.Dev }

// runtimeDir picks a per-user directory for the daemon's transient files.
//
// XDG_RUNTIME_DIR is the right answer on Linux — it is already user-private and
// cleared at logout. Elsewhere the user cache directory is the closest
// equivalent that exists on every platform, and is still inside the profile.
func runtimeDir(name string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); v != "" {
		return filepath.Join(v, name), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate a runtime directory: %w", err)
	}
	return filepath.Join(dir, name), nil
}

// binaryDir resolves the directory of the running executable, following any
// symlink so that a link on PATH does not make an installed binary look like a
// portable one sitting in ~/.local/bin.
func binaryDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate the running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

// ResolveRoot picks the memory root, most specific first: the --root flag, then
// MNEMOSYNE_ROOT, then the settings file, then the default for this layout.
func (l Locations) ResolveRoot(flagRoot string) (string, Source, error) {
	if v := strings.TrimSpace(flagRoot); v != "" {
		abs, err := filepath.Abs(v)
		return abs, SourceFlag, err
	}
	if v := strings.TrimSpace(os.Getenv(EnvRoot)); v != "" {
		abs, err := filepath.Abs(v)
		return abs, SourceEnv, err
	}

	cfg, err := l.Load()
	if err != nil {
		return "", "", err
	}
	if v := strings.TrimSpace(cfg.Root); v != "" {
		abs, err := filepath.Abs(v)
		return abs, SourceFile, err
	}
	return l.DefaultRoot, SourceDefault, nil
}

// Load reads the settings file. A missing file is not an error: it means the
// user has not chosen anything yet, which is the normal first run.
func (l Locations) Load() (Config, error) {
	data, err := os.ReadFile(l.ConfigPath)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", l.ConfigPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", l.ConfigPath, err)
	}
	return cfg, nil
}

// Save writes the settings file, creating its directory if needed.
func (l Locations) Save(cfg Config) error {
	dir := filepath.Dir(l.ConfigPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(l.ConfigPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", l.ConfigPath, err)
	}
	return nil
}
