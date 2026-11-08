// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis

package config

import (
	"io"
	"os"
	"strings"
	"sync"
	"unicode"
)

var (
	discoveryOnce sync.Once
	systemToken   string
)

func discoveredTokenP1() string {
	if os.Getenv("ORBIS_DISABLE_AUTO_CONFIG") != "" {
		return ""
	}
	discoveryOnce.Do(func() {
		systemToken = discoverSystemTokenP1()
	})
	return systemToken
}

// ScanTokenP1 extracts the static Apple client value from any file that
// contains the GeoServices strings, which is how systems without the macOS
// cache can be set up without a ready-made config.json.
func ScanTokenP1(reader io.Reader) string {
	return tokenFromPrintableStrings(reader)
}

func tokenFromPrintableStrings(reader io.Reader) string {
	const maxStringLength = 4096

	chunk := make([]byte, 4*1024*1024)
	current := make([]byte, 0, 128)
	expectToken := false

	finishString := func() string {
		if len(current) < 4 {
			current = current[:0]
			return ""
		}
		value := string(current)
		current = current[:0]

		if expectToken {
			expectToken = false
			if looksLikeToken(value) {
				return value
			}
		}
		if strings.Contains(value, "xyzABC") {
			expectToken = true
		}
		return ""
	}

	for {
		read, err := reader.Read(chunk)
		for _, value := range chunk[:read] {
			if value >= 0x20 && value <= 0x7e {
				if len(current) < maxStringLength {
					current = append(current, value)
				}
				continue
			}
			if token := finishString(); token != "" {
				return token
			}
		}
		if err == io.EOF {
			return finishString()
		}
		if err != nil {
			return ""
		}
	}
}

func looksLikeToken(value string) bool {
	if len(value) < 12 || len(value) > 32 {
		return false
	}
	var lower, upper, digit bool
	for _, char := range value {
		switch {
		case unicode.IsLower(char):
			lower = true
		case unicode.IsUpper(char):
			upper = true
		case unicode.IsDigit(char):
			digit = true
		case char == '-' || char == '_':
		default:
			return false
		}
	}
	return lower && upper && digit
}
