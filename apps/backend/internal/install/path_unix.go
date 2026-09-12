//go:build !windows

package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// targetDir is where the binary is installed.
//
// ~/.local/bin is the per-user convention on both macOS and Linux and is
// already on PATH in most shells, which is what lets install avoid touching
// anyone's dotfiles.
func targetDir(scope Scope) (string, error) {
	if scope == Machine {
		return "/usr/local/bin", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate the home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// ensureOnPath does not modify anything. Shell profiles belong to the user, and
// parsing and rewriting .bashrc or .zshrc to insert a line is both fragile and
// rude. The install directory is already on PATH in a normal setup; when it is
// not, the caller is told the one line to add.
func ensureOnPath(dir string, _ Scope) (bool, string, error) {
	if containsEntry(os.Getenv("PATH"), dir) {
		return false, "", nil
	}
	return false, fmt.Sprintf(
		"%s is not on your PATH. Add this line to your shell profile:\n\n    export PATH=\"%s:$PATH\"",
		dir, dir), nil
}

// removeFromPath has nothing to undo, since ensureOnPath changed nothing.
func removeFromPath(string, Scope) error {
	return nil
}
