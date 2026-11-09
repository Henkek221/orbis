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
	token, _ := scanToken(reader)
	return token
}

// scanToken looks for tokenP1 among the printable strings of a GeoServices
// binary or a dyld shared cache. GeoServices stores it right before its
// "Maps-Tile-…" header names, which is the strong anchor (sure = true). Older
// binaries also keep it right after a base62 alphabet string; that anchor is
// weaker, because other libraries in a shared cache contain similar
// alphabets, so it is only used when the strong one is missing.
func scanToken(reader io.Reader) (token string, sure bool) {
	const maxStringLength = 4096

	chunk := make([]byte, 4*1024*1024)
	current := make([]byte, 0, 128)
	afterAlphabet := false
	previous := ""
	fallback := ""

	// finishString reports true once the strong anchor matched.
	finishString := func() bool {
		if len(current) < 4 {
			current = current[:0]
			return false
		}
		value := string(current)
		current = current[:0]

		if previous != "" && strings.HasPrefix(value, "Maps-Tile-") {
			token, sure = previous, true
			return true
		}
		plausible := looksLikeToken(value) && !looksLikeMangledName(value)
		if afterAlphabet && plausible && fallback == "" {
			fallback = value
		}
		afterAlphabet = strings.Contains(value, "xyzABC")
		previous = ""
		if plausible {
			previous = value
		}
		return false
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
			if finishString() {
				return token, sure
			}
		}
		if err != nil {
			if err == io.EOF && finishString() {
				return token, sure
			}
			return fallback, false
		}
	}
}

// looksLikeMangledName reports whether value reads as an Itanium C++ name
// such as "12SFPWAContext" or "N8Security8RefCountE": length-prefixed
// identifiers that fill the whole string. Such symbols sit next to alphabet
// strings in the shared cache and otherwise pass looksLikeToken.
func looksLikeMangledName(value string) bool {
	body := value
	if len(body) > 2 && body[0] == 'N' && body[len(body)-1] == 'E' {
		body = body[1 : len(body)-1]
	}
	if body == "" || body[0] < '1' || body[0] > '9' {
		return false
	}
	for body != "" {
		digits := 0
		for digits < len(body) && body[digits] >= '0' && body[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits > 3 {
			return false
		}
		length := 0
		for _, digit := range body[:digits] {
			length = length*10 + int(digit-'0')
		}
		rest := body[digits:]
		if length == 0 || length > len(rest) {
			return false
		}
		body = rest[length:]
	}
	return true
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
