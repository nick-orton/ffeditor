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

		cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", src,
			"-codec:a", "libmp3lame", "-qscale:a", "2", dest)
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
