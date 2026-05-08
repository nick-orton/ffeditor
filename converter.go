package main

import (
	"context"
	"fmt"
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

func convertFile(ctx context.Context, src string) tea.Cmd {
	return func() tea.Msg {
		dest := filepath.Join(filepath.Dir(src),
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

// handleConvertMsg handles all conversion lifecycle messages. Returns false if
// the message was not a conversion message.
func handleConvertMsg(m model, msg tea.Msg) (model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case execConvertMsg:
		if !m.ffmpegAvailable {
			m.statusMsg = "ffmpeg not available"
			m.statusIsError = true
			return m, nil, true
		}
		if len(msg.files) == 0 {
			m.statusMsg = "No convertible files selected"
			m.statusIsError = true
			return m, nil, true
		}
		m.convertQueue = msg.files
		m.convertIndex = 0
		m.convertDone = 0
		m.convertSkipped = 0
		m.convertErrors = 0
		ctx, cancel := context.WithCancel(context.Background())
		m.convertCtx = ctx
		m.convertCancel = cancel
		m.statusMsg = fmt.Sprintf("Converting 1/%d...", len(m.convertQueue))
		m.statusIsError = false
		return m, convertFile(m.convertCtx, m.convertQueue[0]), true
	case convertDoneMsg:
		m.convertDone++
		m, cmd := nextConvert(m)
		return m, cmd, true
	case convertSkippedMsg:
		m.convertSkipped++
		m, cmd := nextConvert(m)
		return m, cmd, true
	case convertErrMsg:
		m.convertErrors++
		m, cmd := nextConvert(m)
		return m, cmd, true
	}
	return m, nil, false
}

func nextConvert(m model) (model, tea.Cmd) {
	if m.convertCancelled {
		m.convertCancel = nil
		m.convertCtx = nil
		m.convertCancelled = false
		m.convertQueue = nil
		var teaCmd tea.Cmd
		m.browser, teaCmd = m.browser.changeDir(m.browser.dir)
		return m, teaCmd
	}
	m.convertIndex++
	if m.convertIndex < len(m.convertQueue) {
		m.statusMsg = fmt.Sprintf("Converting %d/%d...", m.convertIndex+1, len(m.convertQueue))
		m.statusIsError = false
		return m, convertFile(m.convertCtx, m.convertQueue[m.convertIndex])
	}
	if m.convertCancel != nil {
		m.convertCancel()
		m.convertCancel = nil
		m.convertCtx = nil
	}
	m.statusMsg = fmt.Sprintf("Conversion complete (%d converted, %d skipped, %d errors)",
		m.convertDone, m.convertSkipped, m.convertErrors)
	m.statusIsError = m.convertErrors > 0
	var teaCmd tea.Cmd
	m.browser, teaCmd = m.browser.changeDir(m.browser.dir)
	return m, teaCmd
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
