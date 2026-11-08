// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package mps

import "testing"

func TestCacheBaseURL(t *testing.T) {
	t.Run("legacy manifest", func(t *testing.T) {
		rm := ResourceManifest{CacheBaseUrl: "https://legacy.example/"}
		if got := rm.CacheBaseURL(); got != "https://legacy.example/" {
			t.Fatalf("CacheBaseURL() = %q", got)
		}
	})

	t.Run("current endpoint table", func(t *testing.T) {
		rm := ResourceManifest{
			UrlInfoSet: &ResourceManifest_URLInfoSet{
				CacheBase: &ResourceManifest_URLInfo{Url: "https://current.example/"},
			},
		}
		if got := rm.CacheBaseURL(); got != "https://current.example/" {
			t.Fatalf("CacheBaseURL() = %q", got)
		}
	})
}
