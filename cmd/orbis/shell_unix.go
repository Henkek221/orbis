// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

//go:build !windows

package main

import "os"

// startedFromShell is false for a double-clicked app, whose stdin is not a
// terminal.
func startedFromShell() bool {
	return isTerminal(os.Stdin)
}
