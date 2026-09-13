package main

import (
	"syscall"
	"unsafe"
)

// detachConsole drops the console window Windows gave this process, but only
// when the process is the sole owner of it.
//
// A console-subsystem binary started by the Scheduled Task, by Explorer, or by
// a GUI app gets a console of its own, which is the black window that sits on
// the desktop for as long as the daemon runs. Started from a terminal, the
// console is shared with the shell — freeing that one would throw away the
// logs the developer is standing there to read.
//
// GetConsoleProcessList tells the two apart: a console created for us has
// exactly one process attached, ours.
func detachConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	list := kernel32.NewProc("GetConsoleProcessList")
	free := kernel32.NewProc("FreeConsole")

	var pids [4]uint32
	n, _, _ := list.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n == 1 {
		free.Call()
	}
}
