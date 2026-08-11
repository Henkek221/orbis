package oth

import (
	"errors"
	"os/exec"
	"runtime"
	"sync"
)

// Apple ships newer map textures as HEIC. Go cannot decode that without a C
// dependency, so exports reuse whatever converter is installed locally.
type heicTool struct {
	binary string
	args   func(src, dst string) []string
}

var (
	heicOnce sync.Once
	heicPath string
	heicArgs func(src, dst string) []string
)

func heicTools() []heicTool {
	tools := []heicTool{
		{"magick", func(src, dst string) []string { return []string{src, dst} }},
		{"heif-convert", func(src, dst string) []string { return []string{src, dst} }},
		{"ffmpeg", func(src, dst string) []string {
			return []string{"-y", "-loglevel", "error", "-i", src, dst}
		}},
	}
	if runtime.GOOS == "darwin" {
		tools = append([]heicTool{{"sips", func(src, dst string) []string {
			return []string{"-s", "format", "jpeg", src, "--out", dst}
		}}}, tools...)
	}
	return tools
}

func findHEICTool() {
	for _, tool := range heicTools() {
		path, err := exec.LookPath(tool.binary)
		if err != nil {
			continue
		}
		heicPath, heicArgs = path, tool.args
		return
	}
}

// HEICConverter names the local tool that can turn HEIC textures into JPEG, or
// returns an empty string when none is installed.
func HEICConverter() string {
	heicOnce.Do(findHEICTool)
	return heicPath
}

// ConvertHEIC writes src as a JPEG to dst using a locally installed converter.
func ConvertHEIC(src, dst string) error {
	if HEICConverter() == "" {
		return errors.New("no local HEIC converter found (install ImageMagick or ffmpeg)")
	}
	return exec.Command(heicPath, heicArgs(src, dst)...).Run()
}
