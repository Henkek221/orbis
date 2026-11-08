//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var procGetConsoleProcessList = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")

// startedFromShell is false when Explorer created a console just for this
// process, which then is the only one attached to it.
func startedFromShell() bool {
	var ids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&ids[0])), uintptr(len(ids)))
	return n > 1
}
