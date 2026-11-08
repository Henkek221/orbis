// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

// Command orbis exports Apple's 3D map mesh for an area as OBJ and/or a
// Minecraft world, from the command line or a local browser UI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Henkek221/orbis/internal/server"
	"github.com/Henkek221/orbis/pkg/orbis"
)

// version is set by the release build with -ldflags "-X main.version=…".
var version = "dev"

const usage = `Orbis exports Apple's 3D map mesh for real places.

Usage:
  orbis <command> [flags]

Commands:
  export     export an area as GLB (any engine), OBJ or a Minecraft world
  coverage   show which tiles of an area have 3D data
  search     find a place and print its coordinates
  regions    list the 3D regions the service announces
  config     show, import or scan the runtime configuration
  ui         open the browser interface
  version    print the version

An area is one of:
  --bbox W,S,E,N           west, south, east, north in degrees
  --center LAT,LON         a point, sized with --size
  --place "Name"           a geocoded place, sized with --size
  --size 250 | 300x200     metres around a point (default 250)

Examples:
  orbis export --place "Brandenburger Tor" --size 300
  orbis export --center 48.8584,2.2945 --format glb,obj --out ./exports
  orbis coverage --place "Times Square"
  orbis config import ~/mead/iOS-18.2/mead/config.json

Data (config.json and cache) lives in --home, $ORBIS_HOME, the current
directory if it has a config.json, or the user config directory.
Run "orbis <command> -h" for the flags of a command.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		if !startedFromShell() {
			// Double-clicked app or launcher script: behave like the old UI binary.
			return runUI(nil)
		}
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	command, rest := args[0], args[1:]
	switch command {
	case "export":
		return runExport(rest)
	case "coverage":
		return runCoverage(rest)
	case "search":
		return runSearch(rest)
	case "regions":
		return runRegions(rest)
	case "config":
		return runConfig(rest)
	case "ui", "serve":
		return runUI(rest)
	case "version", "--version", "-v":
		fmt.Println("orbis", version)
		return 0
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "orbis: unknown command %q\n\n%s", command, usage)
		return 2
	}
}

func runUI(args []string) int {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	addr := fs.String("addr", envOr("FLYOVER_UI_ADDR", ":8787"), "listen address")
	home := fs.String("home", "", "data directory (config.json, cache, exports)")
	noBrowser := fs.Bool("no-browser", os.Getenv("ORBIS_NO_BROWSER") != "", "do not open the browser")
	if _, err := parseArgs(fs, args); err != nil {
		return 2
	}
	dir := *home
	if dir == "" && startedFromShell() {
		// From a terminal, share data with the other commands instead of
		// writing next to the binary.
		resolved, err := resolveHome("")
		if err != nil {
			return fail(err)
		}
		dir = resolved
	}
	if err := server.Run(server.Options{Addr: *addr, Home: dir, OpenBrowser: !*noBrowser}); err != nil {
		return fail(err)
	}
	return 0
}

// resolveHome picks the data directory: the flag, $ORBIS_HOME, the working
// directory when it already holds a config.json, else <user config dir>/Orbis.
func resolveHome(flagValue string) (string, error) {
	if flagValue != "" {
		return filepath.Abs(flagValue)
	}
	if env := os.Getenv("ORBIS_HOME"); env != "" {
		return filepath.Abs(env)
	}
	if _, err := os.Stat("config.json"); err == nil {
		return os.Getwd()
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Orbis"), nil
}

type paths struct {
	Home   string
	Config string
	Cache  string
}

func homePaths(flagValue string) (paths, error) {
	home, err := resolveHome(flagValue)
	if err != nil {
		return paths{}, err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return paths{}, err
	}
	return paths{Home: home, Config: filepath.Join(home, "config.json"), Cache: filepath.Join(home, "cache")}, nil
}

func openClient(p paths) (*orbis.Client, error) {
	client, err := orbis.Open(orbis.Options{ConfigPath: p.Config, CacheDir: p.Cache})
	if errors.Is(err, orbis.ErrNotConfigured) {
		return nil, fmt.Errorf("%w\n  on Windows or Linux: orbis config import <config.json or kvasir output folder>", err)
	}
	return client, err
}

// parseArgs parses flags and positional arguments in any order.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	fs.SetOutput(os.Stderr)
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "orbis:", err)
	return 1
}

var stdout io.Writer = os.Stdout
