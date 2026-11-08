// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Henkek221/orbis/pkg/orbis"
)

const configUsage = `Usage:
  orbis config [status]        show where Orbis reads its configuration and test it
  orbis config import PATH     use a config.json, or a kvasir output folder
  orbis config scan PATH       read the client value from a GeoServices binary
  orbis config write FILE      write the working config to FILE for another computer
  orbis config path            print the data directory

Flags:
  --home DIR    data directory (default: $ORBIS_HOME, ./config.json, user config dir)

On macOS no config is needed: Orbis finds the value in the system. On Windows
and Linux, create one on any Mac with "orbis config write config.json" or with
kvasir (github.com/Henkek221/kvasir), then import it.
`

func runConfig(args []string) int {
	flags := flag.NewFlagSet("config", flag.ContinueOnError)
	home := flags.String("home", "", "data directory")
	flags.Usage = func() { fmt.Fprint(os.Stderr, configUsage) }
	positional, err := parseArgs(flags, args)
	if err != nil {
		return 2
	}
	sub := "status"
	if len(positional) > 0 {
		sub, positional = positional[0], positional[1:]
	}
	p, err := homePaths(*home)
	if err != nil {
		return fail(err)
	}

	switch sub {
	case "status":
		return configStatus(p)
	case "path":
		fmt.Fprintln(stdout, p.Home)
		return 0
	case "import", "scan":
		if len(positional) != 1 {
			fmt.Fprint(os.Stderr, configUsage)
			return 2
		}
		cfg, source, err := configFromPath(positional[0], sub == "scan")
		if err != nil {
			return fail(err)
		}
		return installConfig(p, cfg, source)
	case "write":
		if len(positional) != 1 {
			fmt.Fprint(os.Stderr, configUsage)
			return 2
		}
		cfg, err := orbis.LoadConfig(p.Config)
		if err != nil {
			return fail(err)
		}
		if !cfg.IsValid() {
			return fail(orbis.ErrNotConfigured)
		}
		if err := orbis.SaveConfig(positional[0], cfg); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "wrote %s (client value %s). Keep it private: never commit or post it.\n", positional[0], mask(cfg.TokenP1))
		return 0
	default:
		fmt.Fprint(os.Stderr, configUsage)
		return 2
	}
}

func configStatus(p paths) int {
	fmt.Fprintln(stdout, "home:    ", p.Home)
	fileCfg, fileErr := readConfigFile(p.Config)
	switch {
	case fileErr == nil:
		fmt.Fprintln(stdout, "file:    ", p.Config)
	case errors.Is(fileErr, fs.ErrNotExist):
		fmt.Fprintln(stdout, "file:     none")
	default:
		fmt.Fprintln(stdout, "file:     unreadable:", fileErr)
	}

	cfg, err := orbis.LoadConfig(p.Config)
	if err != nil {
		return fail(err)
	}
	source := "local system (macOS GeoServices)"
	switch {
	case os.Getenv("FLYOVER_TOKEN_P1") != "":
		source = "FLYOVER_TOKEN_P1"
	case fileErr == nil && fileCfg.TokenP1 != "":
		source = "config.json"
	}
	if !cfg.IsValid() {
		fmt.Fprintln(stdout, "token:    missing")
		fmt.Fprint(os.Stderr, "\n"+configUsage)
		return 1
	}
	fmt.Fprintf(stdout, "token:    %s from %s\n", mask(cfg.TokenP1), source)

	if _, err := orbis.Connect(cfg, p.Cache); err != nil {
		fmt.Fprintln(stdout, "service:  failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, "service:  connected")
	return 0
}

// configFromPath accepts a config.json, a kvasir output folder, or (scan) a
// GeoServices binary or dyld shared cache.
func configFromPath(path string, scan bool) (orbis.Config, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return orbis.Config{}, "", err
	}
	if info.IsDir() {
		found, err := findInKvasirOutput(path)
		if err != nil {
			return orbis.Config{}, "", err
		}
		path = found
		scan = filepath.Base(found) != "config.json"
	}

	if !scan {
		cfg, err := readConfigFile(path)
		if err != nil {
			return orbis.Config{}, "", fmt.Errorf("%s is not a Orbis config.json: %w", path, err)
		}
		return cfg, path, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return orbis.Config{}, "", err
	}
	defer f.Close()
	token := orbis.ScanToken(f)
	if token == "" {
		return orbis.Config{}, "", fmt.Errorf("no Apple client value found in %s; use the GeoServices binary", path)
	}
	return orbis.ConfigFromToken(token), path, nil
}

// findInKvasirOutput looks for kvasir's mead/config.json, then for the
// extracted GeoServices binary, newest runtime first.
func findInKvasirOutput(root string) (string, error) {
	var configs, binaries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch d.Name() {
		case "config.json":
			configs = append(configs, path)
		case "GeoServices":
			binaries = append(binaries, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(configs) > 0 {
		return configs[len(configs)-1], nil
	}
	if len(binaries) > 0 {
		return binaries[len(binaries)-1], nil
	}
	return "", fmt.Errorf("no config.json or GeoServices in %s; run kvasir with -packs mead first", root)
}

func installConfig(p paths, cfg orbis.Config, source string) int {
	if err := orbis.ValidateConfig(cfg); err != nil {
		return fail(err)
	}
	if _, err := orbis.Connect(cfg, p.Cache); err != nil {
		return fail(fmt.Errorf("the service rejected this configuration: %w", err))
	}
	if err := orbis.SaveConfig(p.Config, cfg); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stdout, "configured from %s\nsaved %s (client value %s)\n", source, p.Config, mask(cfg.TokenP1))
	return 0
}

func readConfigFile(path string) (orbis.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return orbis.Config{}, err
	}
	defer f.Close()
	return orbis.ParseConfig(f)
}

func mask(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 6 {
		return strings.Repeat("•", len(value))
	}
	return value[:3] + strings.Repeat("•", len(value)-6) + value[len(value)-3:]
}
