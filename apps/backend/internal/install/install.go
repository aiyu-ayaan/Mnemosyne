// Package install puts the mnemosyne binary somewhere permanent and makes it
// reachable from any terminal.
//
// Per-user is the default and never needs admin rights. Machine scope exists
// only to put the binary on the PATH of every account on a shared machine.
package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Scope selects who the install is for.
type Scope int

const (
	// User installs into the current account and needs no elevation.
	User Scope = iota
	// Machine installs for every account and requires admin rights.
	Machine
)

func (s Scope) String() string {
	if s == Machine {
		return "machine"
	}
	return "user"
}

// Options controls an install.
type Options struct {
	Scope Scope

	// SkipPath leaves PATH alone, for a user who manages it themselves.
	SkipPath bool
}

// Result describes what an install actually changed, so the caller can report
// it accurately rather than claiming a fixed list of steps.
type Result struct {
	BinaryPath  string
	Dir         string
	PathAdded   bool
	PathAlready bool
	PathNote    string
}

// binaryName is the file the binary is installed as.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "mnemosyne.exe"
	}
	return "mnemosyne"
}

// Dir is where the binary is installed for a scope.
func Dir(scope Scope) (string, error) {
	return targetDir(scope)
}

// Run installs the currently running binary.
func Run(opts Options) (Result, error) {
	source, err := os.Executable()
	if err != nil {
		return Result{}, fmt.Errorf("locate the running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	}

	dir, err := targetDir(opts.Scope)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create %s: %w", dir, err)
	}

	target := filepath.Join(dir, binaryName())
	res := Result{BinaryPath: target, Dir: dir}

	// Installing the binary over itself is a no-op, not an error: re-running
	// install to fix up PATH is a reasonable thing to do.
	if !sameFile(source, target) {
		if err := copyExecutable(source, target); err != nil {
			return res, err
		}
	}

	if opts.SkipPath {
		return res, nil
	}

	added, note, err := ensureOnPath(dir, opts.Scope)
	if err != nil {
		return res, err
	}
	res.PathAdded = added
	res.PathAlready = !added && note == ""
	res.PathNote = note
	return res, nil
}

// Remove undoes an install: the PATH entry first, then the binary.
func Remove(scope Scope) (Result, error) {
	dir, err := targetDir(scope)
	if err != nil {
		return Result{}, err
	}
	target := filepath.Join(dir, binaryName())
	res := Result{BinaryPath: target, Dir: dir}

	if err := removeFromPath(dir, scope); err != nil {
		return res, err
	}

	// A running Windows binary cannot delete itself, so this is expected to
	// fail when uninstalling with the installed copy. Say so rather than
	// reporting a generic error.
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("remove %s: %w — if this is the running binary, delete it manually", target, err)
	}
	return res, nil
}

func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

// copyExecutable writes the binary through a temp file and renames it, so an
// interrupted install cannot leave a truncated executable on PATH.
func copyExecutable(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(target), ".mnemosyne-install-*")
	if err != nil {
		return fmt.Errorf("create a temp file in %s: %w", filepath.Dir(target), err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("copy the binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close the temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("make the binary executable: %w", err)
	}

	// Windows refuses to rename over an existing file, and refuses to remove
	// one that is running. Moving it aside works in both cases and lets an
	// upgrade replace a binary that is currently in use.
	if _, err := os.Stat(target); err == nil {
		old := target + ".old"
		os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			os.Remove(tmpName)
			return fmt.Errorf("replace %s: %w", target, err)
		}
		defer os.Remove(old)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("install to %s: %w", target, err)
	}
	return nil
}

// --- PATH string handling, kept pure so it can be tested directly ---

// pathSeparator is the character separating PATH entries.
func pathSeparator() string {
	return string(os.PathListSeparator)
}

func containsEntry(current, dir string) bool {
	for part := range strings.SplitSeq(current, pathSeparator()) {
		if equalPath(part, dir) {
			return true
		}
	}
	return false
}

// equalPath compares two PATH entries, ignoring a trailing separator, quotes,
// and — on Windows — case.
func equalPath(a, b string) bool {
	a = normalisePath(a)
	b = normalisePath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func normalisePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `"`)
	if len(p) > 1 {
		p = strings.TrimRight(p, `\/`)
	}
	return filepath.Clean(p)
}
