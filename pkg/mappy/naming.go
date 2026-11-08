package mappy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// ExportName turns a human name into a folder-safe slug. Without a name it
// falls back to the centre coordinates and today's date.
func ExportName(name string, b Bounds) string {
	if slug := Slugify(name); slug != "" {
		return slug
	}
	lat, lon := b.Center()
	return fmt.Sprintf("area-%.4f-%.4f-%s", lat, lon, time.Now().Format("2006-01-02"))
}

// Slugify lowercases letters and digits, joins everything else with single
// dashes and caps the result at 80 runes. Non-ASCII letters are kept.
func Slugify(value string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.TrimSpace(value) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	runes := []rune(name)
	if len(runes) > 80 {
		name = strings.TrimRight(string(runes[:80]), "-")
	}
	return name
}

// AvailableOutputPath returns root/name, or root/name-2, -3 … if that folder
// already exists, so an export never overwrites an earlier one.
func AvailableOutputPath(root, name string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(root, name)
	for suffix := 2; ; suffix++ {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		path = filepath.Join(root, fmt.Sprintf("%s-%d", name, suffix))
	}
}
