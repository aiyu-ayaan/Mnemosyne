package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// targetDir is where the binary is installed.
//
// The per-user location mirrors what other CLI tools use on Windows and needs
// no elevation; the machine location does.
func targetDir(scope Scope) (string, error) {
	if scope == Machine {
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		return filepath.Join(programFiles, "Mnemosyne"), nil
	}

	local, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return "", fmt.Errorf("locate the local app data directory: %w", err)
	}
	return filepath.Join(local, "Programs", "Mnemosyne"), nil
}

// pathKey opens the registry key holding PATH for a scope.
func pathKey(scope Scope, access uint32) (registry.Key, error) {
	if scope == Machine {
		return registry.OpenKey(registry.LOCAL_MACHINE,
			`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, access)
	}
	return registry.OpenKey(registry.CURRENT_USER, `Environment`, access)
}

// ensureOnPath adds dir to the PATH registry value if it is not already there.
func ensureOnPath(dir string, scope Scope) (bool, string, error) {
	key, err := pathKey(scope, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		if scope == Machine {
			return false, "", fmt.Errorf("open the machine environment key: %w — run this from an elevated prompt", err)
		}
		return false, "", fmt.Errorf("open the user environment key: %w", err)
	}
	defer key.Close()

	current, valueType, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return false, "", fmt.Errorf("read the current PATH: %w", err)
	}
	// A PATH containing %VARIABLES% must be written back as REG_EXPAND_SZ, or
	// the variables stop expanding and the user's PATH quietly breaks.
	if valueType != registry.EXPAND_SZ {
		valueType = registry.SZ
	}

	updated, changed := addEntry(current, dir)
	if !changed {
		return false, "", nil
	}

	if valueType == registry.EXPAND_SZ {
		err = key.SetExpandStringValue("Path", updated)
	} else {
		err = key.SetStringValue("Path", updated)
	}
	if err != nil {
		return false, "", fmt.Errorf("update PATH: %w", err)
	}

	broadcastEnvironmentChange()
	return true, "", nil
}

// removeFromPath drops dir from the PATH registry value.
func removeFromPath(dir string, scope Scope) error {
	key, err := pathKey(scope, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open the environment key: %w", err)
	}
	defer key.Close()

	current, valueType, err := key.GetStringValue("Path")
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the current PATH: %w", err)
	}

	updated, changed := removeEntry(current, dir)
	if !changed {
		return nil
	}

	if valueType == registry.EXPAND_SZ {
		err = key.SetExpandStringValue("Path", updated)
	} else {
		err = key.SetStringValue("Path", updated)
	}
	if err != nil {
		return fmt.Errorf("update PATH: %w", err)
	}

	broadcastEnvironmentChange()
	return nil
}

// broadcastEnvironmentChange tells running processes that the environment
// changed. Without it, open terminals and Explorer keep the old PATH until the
// next logon and the install looks like it did nothing.
func broadcastEnvironmentChange() {
	const (
		hwndBroadcast   = 0xFFFF
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)

	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("SendMessageTimeoutW")

	env, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	// Failure here is not worth reporting: PATH is already updated, and the
	// worst case is that the user opens a new terminal.
	proc.Call(hwndBroadcast, wmSettingChange, 0,
		uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 5000,
		uintptr(unsafe.Pointer(&result)))
}

// --- PATH string edits. Only Windows rewrites PATH, so these live here. ---

// addEntry appends dir to a PATH string, reporting whether it changed anything.
// Adding twice must leave one entry, so that repeated installs do not grow PATH.
func addEntry(current, dir string) (string, bool) {
	if containsEntry(current, dir) {
		return current, false
	}
	if strings.TrimSpace(current) == "" {
		return dir, true
	}
	return strings.TrimSuffix(current, pathSeparator()) + pathSeparator() + dir, true
}

// removeEntry drops every occurrence of dir from a PATH string.
func removeEntry(current, dir string) (string, bool) {
	parts := strings.Split(current, pathSeparator())
	kept := make([]string, 0, len(parts))
	changed := false
	for _, part := range parts {
		if part == "" {
			continue
		}
		if equalPath(part, dir) {
			changed = true
			continue
		}
		kept = append(kept, part)
	}
	if !changed {
		return current, false
	}
	return strings.Join(kept, pathSeparator()), true
}
