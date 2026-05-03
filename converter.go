package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type convertDoneMsg struct{ src, dest string }
type convertErrMsg struct {
	src string
	err error
}
type convertSkippedMsg struct{ src string }
type convertProgressMsg struct{ current, total int }

var convertExts = map[string]bool{
	".opus": true, ".m4a": true, ".ogg": true,
}

func convertFile(ctx context.Context, src, destDir string) tea.Cmd {
	return func() tea.Msg {
		dest := filepath.Join(destDir,
			strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))+".mp3")

		if _, err := os.Stat(dest); err == nil {
			return convertSkippedMsg{src}
		}

		// ogg/opus store user tags in stream-level metadata (Vorbis Comments),
		// so they must be mapped to global output metadata explicitly.
		// m4a stores tags at the container level, so -map_metadata 0 suffices.
		metaArgs := []string{"-map_metadata", "0"}
		ext := strings.ToLower(filepath.Ext(src))
		if ext == ".opus" || ext == ".ogg" {
			metaArgs = []string{"-map_metadata:g", "0:s:0"}
		}

		args := append([]string{"-y", "-i", src}, metaArgs...)
		args = append(args, "-codec:a", "libmp3lame", "-qscale:a", "2", dest)
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Run(); err != nil {
			return convertErrMsg{src, err}
		}
		return convertDoneMsg{src, dest}
	}
}

// buildConvertList collects convertible files from the given entries.
// Directories are walked recursively. Duplicate paths are removed.
func buildConvertList(entries []os.DirEntry, dir string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			_ = filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(d.Name()))
				if convertExts[ext] && !seen[path] {
					seen[path] = true
					result = append(result, path)
				}
				return nil
			})
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if convertExts[ext] && !seen[fullPath] {
				seen[fullPath] = true
				result = append(result, fullPath)
			}
		}
	}

	return result
}
