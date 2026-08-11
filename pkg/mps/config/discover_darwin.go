//go:build darwin

package config

import (
	"os"
	"runtime"
)

func discoverSystemTokenP1() string {
	candidates := []string{
		"/System/Library/PrivateFrameworks/GeoServices.framework/GeoServices",
		"/System/Library/PrivateFrameworks/GeoServices.framework/Versions/A/GeoServices",
	}

	const cacheRoot = "/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld/"
	if runtime.GOARCH == "arm64" {
		candidates = append(candidates,
			cacheRoot+"dyld_shared_cache_arm64e.01",
			cacheRoot+"dyld_shared_cache_arm64e",
		)
	} else {
		candidates = append(candidates,
			cacheRoot+"dyld_shared_cache_x86_64",
			cacheRoot+"dyld_shared_cache_x86_64.01",
		)
	}

	for _, candidate := range candidates {
		file, err := os.Open(candidate)
		if err != nil {
			continue
		}
		token := tokenFromPrintableStrings(file)
		_ = file.Close()
		if token != "" {
			return token
		}
	}
	return ""
}
