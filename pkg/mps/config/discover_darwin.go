// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

//go:build darwin

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	dyldCacheRoot  = "/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld/"
	simGeoServices = "Contents/Resources/RuntimeRoot/System/Library/PrivateFrameworks/GeoServices.framework/GeoServices"
)

// discoverSystemTokenP1 reads tokenP1 from this Mac: the GeoServices binary,
// every dyld shared subcache (macOS 27 moved GeoServices' strings out of
// .01), and finally the GeoServices binaries of installed iOS simulator
// runtimes. The first value found by the strong anchor wins; a value found
// only by the weaker alphabet anchor is used when nothing better turns up.
func discoverSystemTokenP1() string {
	candidates := []string{
		"/System/Library/PrivateFrameworks/GeoServices.framework/GeoServices",
		"/System/Library/PrivateFrameworks/GeoServices.framework/Versions/A/GeoServices",
	}
	candidates = append(candidates, dyldSubcaches()...)
	candidates = append(candidates, simulatorGeoServices()...)

	fallback := ""
	for _, candidate := range candidates {
		file, err := os.Open(candidate)
		if err != nil {
			continue
		}
		token, sure := scanToken(file)
		_ = file.Close()
		if sure {
			return token
		}
		if fallback == "" {
			fallback = token
		}
	}
	return fallback
}

// dyldSubcaches lists the main dyld shared cache and its numbered subcaches
// (dyld_shared_cache_arm64e, .01, .02, …) in order. Symbol maps, atlases and
// the .dylddata/.dyldreadonly/.dyldlinkedit parts hold no strings and are
// skipped.
func dyldSubcaches() []string {
	base := "dyld_shared_cache_x86_64"
	if runtime.GOARCH == "arm64" {
		base = "dyld_shared_cache_arm64e"
	}
	matches, _ := filepath.Glob(dyldCacheRoot + base + "*")

	type subcache struct {
		path  string
		index int
	}
	var caches []subcache
	for _, match := range matches {
		name := filepath.Base(match)
		if name == base {
			caches = append(caches, subcache{match, 0})
			continue
		}
		suffix, ok := strings.CutPrefix(name, base+".")
		if !ok {
			continue
		}
		index, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		caches = append(caches, subcache{match, index})
	}
	sort.Slice(caches, func(i, j int) bool { return caches[i].index < caches[j].index })

	paths := make([]string, len(caches))
	for i, cache := range caches {
		paths[i] = cache.path
	}
	return paths
}

// simulatorGeoServices lists the standalone GeoServices binaries of installed
// iOS simulator runtimes, newest name first. Runtimes that ship only a
// simulator shared cache have none and are not listed.
func simulatorGeoServices() []string {
	patterns := []string{
		"/Library/Developer/CoreSimulator/Volumes/*/Library/Developer/CoreSimulator/Profiles/Runtimes/*.simruntime/" + simGeoServices,
		"/Library/Developer/CoreSimulator/Profiles/Runtimes/*.simruntime/" + simGeoServices,
	}
	var paths []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		paths = append(paths, matches...)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths
}
