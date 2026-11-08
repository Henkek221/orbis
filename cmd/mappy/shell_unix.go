//go:build !windows

package main

import "os"

// startedFromShell is false for a double-clicked app, whose stdin is not a
// terminal.
func startedFromShell() bool {
	return isTerminal(os.Stdin)
}
