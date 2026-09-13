package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstalledLayout(t *testing.T) {
	loc, err := Detect(false)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if loc.Portable {
		t.Fatal("Portable = true; the test binary has no marker file beside it")
	}
	// The settings file must not sit inside the memory root, since it is what
	// says where that root is.
	if rel, err := filepath.Rel(loc.DefaultRoot, loc.ConfigPath); err == nil &&
		rel != ".." && filepath.IsLocal(rel) {
		t.Errorf("config %q is inside the default root %q", loc.ConfigPath, loc.DefaultRoot)
	}
}

func TestDetectForcedPortable(t *testing.T) {
	loc, err := Detect(true)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !loc.Portable {
		t.Fatal("Portable = false despite --portable")
	}
	if filepath.Dir(loc.ConfigPath) != loc.BinDir {
		t.Errorf("config %q is not beside the binary in %q", loc.ConfigPath, loc.BinDir)
	}
	if filepath.Dir(loc.DefaultRoot) != loc.BinDir {
		t.Errorf("root %q is not beside the binary in %q", loc.DefaultRoot, loc.BinDir)
	}
}

// portableAt builds a Locations for a fake portable folder, which is how the
// resolution tests avoid depending on where the test binary happens to live.
func portableAt(dir string) Locations {
	return Locations{
		Portable:    true,
		BinDir:      dir,
		ConfigPath:  filepath.Join(dir, "config.json"),
		DefaultRoot: filepath.Join(dir, "memories"),
	}
}

func TestResolveRootPrecedence(t *testing.T) {
	dir := t.TempDir()
	loc := portableAt(dir)

	t.Run("default", func(t *testing.T) {
		t.Setenv(EnvRoot, "")
		got, source, err := loc.ResolveRoot("")
		if err != nil {
			t.Fatalf("ResolveRoot: %v", err)
		}
		if got != loc.DefaultRoot || source != SourceDefault {
			t.Errorf("got %q from %q, want the default %q", got, source, loc.DefaultRoot)
		}
	})

	t.Run("config file beats default", func(t *testing.T) {
		t.Setenv(EnvRoot, "")
		fromFile := filepath.Join(dir, "from-file")
		if err := loc.Save(Config{Root: fromFile}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		t.Cleanup(func() { os.Remove(loc.ConfigPath) })

		got, source, err := loc.ResolveRoot("")
		if err != nil {
			t.Fatalf("ResolveRoot: %v", err)
		}
		if got != fromFile || source != SourceFile {
			t.Errorf("got %q from %q, want %q from the config file", got, source, fromFile)
		}
	})

	t.Run("env beats config file", func(t *testing.T) {
		fromEnv := filepath.Join(dir, "from-env")
		t.Setenv(EnvRoot, fromEnv)
		if err := loc.Save(Config{Root: filepath.Join(dir, "from-file")}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		t.Cleanup(func() { os.Remove(loc.ConfigPath) })

		got, source, err := loc.ResolveRoot("")
		if err != nil {
			t.Fatalf("ResolveRoot: %v", err)
		}
		if got != fromEnv || source != SourceEnv {
			t.Errorf("got %q from %q, want %q from the environment", got, source, fromEnv)
		}
	})

	t.Run("flag beats everything", func(t *testing.T) {
		fromFlag := filepath.Join(dir, "from-flag")
		t.Setenv(EnvRoot, filepath.Join(dir, "from-env"))
		if err := loc.Save(Config{Root: filepath.Join(dir, "from-file")}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		t.Cleanup(func() { os.Remove(loc.ConfigPath) })

		got, source, err := loc.ResolveRoot(fromFlag)
		if err != nil {
			t.Fatalf("ResolveRoot: %v", err)
		}
		if got != fromFlag || source != SourceFlag {
			t.Errorf("got %q from %q, want %q from the flag", got, source, fromFlag)
		}
	})
}

func TestMissingConfigFileIsNotAnError(t *testing.T) {
	loc := portableAt(t.TempDir())

	cfg, err := loc.Load()
	if err != nil {
		t.Fatalf("Load on a first run must succeed: %v", err)
	}
	if cfg.Root != "" {
		t.Errorf("Root = %q, want empty", cfg.Root)
	}
}

func TestCorruptConfigFileIsReported(t *testing.T) {
	loc := portableAt(t.TempDir())
	if err := os.WriteFile(loc.ConfigPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Silently falling back to the default would point the user at an empty
	// memory root and look like their memories vanished.
	if _, err := loc.Load(); err == nil {
		t.Fatal("Load succeeded on a corrupt config file")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	loc := portableAt(t.TempDir())
	want := filepath.Join(t.TempDir(), "elsewhere")

	if err := loc.Save(Config{Root: want}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := loc.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Root != want {
		t.Errorf("Root = %q, want %q", got.Root, want)
	}
}

// TestDetectDevLayout is the guarantee that matters for development mode: a
// dev run must not touch any path the installed copy uses.
func TestDetectDevLayout(t *testing.T) {
	installed, err := Detect(false)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	t.Setenv(EnvDev, "1")
	dev, err := Detect(false)
	if err != nil {
		t.Fatalf("Detect dev: %v", err)
	}

	if !dev.Dev || !dev.Isolated() {
		t.Fatalf("dev layout not marked as dev/isolated: %+v", dev)
	}
	if dev.ConfigPath == installed.ConfigPath {
		t.Errorf("dev shares the config file: %s", dev.ConfigPath)
	}
	if dev.DefaultRoot == installed.DefaultRoot {
		t.Errorf("dev shares the memory root: %s", dev.DefaultRoot)
	}
	if dev.RuntimeDir == installed.RuntimeDir {
		t.Errorf("dev shares the runtime directory: %s", dev.RuntimeDir)
	}
}

func TestDetectLayoutExplicit(t *testing.T) {
	installed, err := DetectLayout(false, false)
	if err != nil {
		t.Fatalf("DetectLayout false: %v", err)
	}
	dev, err := DetectLayout(false, true)
	if err != nil {
		t.Fatalf("DetectLayout true: %v", err)
	}
	if !dev.Dev || !dev.Isolated() {
		t.Fatalf("explicit dev layout not marked dev: %+v", dev)
	}
	if dev.ConfigPath == installed.ConfigPath {
		t.Errorf("explicit dev shares config path: %s", dev.ConfigPath)
	}
}
