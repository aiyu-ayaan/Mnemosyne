//go:build !windows

package main

// detachConsole has nothing to do: a Unix daemon inherits whatever stdio its
// launcher gave it, and no window is ever created.
func detachConsole() {}
