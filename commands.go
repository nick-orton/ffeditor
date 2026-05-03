package main

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type cmdbarModel struct {
	input  string
	active bool
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}

func parseCommand(input string) (cmd string, args []string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func (m cmdbarModel) Update(msg tea.Msg) (cmdbarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.input = ""
			m.active = false
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.Runes) == 1 {
				m.input += string(msg.Runes)
			}
		}
	}
	return m, nil
}

// tabComplete attempts directory tab-completion for the "cd" command.
// It returns the updated input string. browserDir is used as the base
// for resolving relative paths.
func tabComplete(input, browserDir string) string {
	trimmed := strings.TrimLeft(input, " ")
	if !strings.HasPrefix(trimmed, "cd") {
		return input
	}
	after := strings.TrimPrefix(trimmed, "cd")
	if after != "" && after[0] != ' ' {
		return input // e.g. "cdfoo"
	}
	partial := expandTilde(strings.TrimLeft(after, " "))

	// Split partial into the directory to list and the name prefix to match.
	var listDir, prefix string
	if partial == "" || strings.HasSuffix(partial, "/") {
		listDir = partial
		prefix = ""
	} else {
		dir := filepath.Dir(partial)
		base := filepath.Base(partial)
		switch dir {
		case "/":
			listDir = "/"
		case ".":
			listDir = ""
		default:
			listDir = dir + "/"
		}
		prefix = base
	}

	// Resolve listDir to an absolute path for reading.
	var absListDir string
	if listDir == "" {
		absListDir = browserDir
	} else if filepath.IsAbs(listDir) {
		absListDir = listDir
	} else {
		absListDir = filepath.Join(browserDir, listDir)
	}

	entries, err := os.ReadDir(absListDir)
	if err != nil {
		return input
	}

	var matches []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		if e.IsDir() {
			matches = append(matches, e.Name())
			continue
		}
		// Follow symlinks: include if the target is a directory.
		if e.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(filepath.Join(absListDir, e.Name()))
			if err == nil && info.IsDir() {
				matches = append(matches, e.Name())
			}
		}
	}
	if len(matches) == 0 {
		return input
	}

	completed := longestCommonPrefix(matches)
	result := "cd " + listDir + completed
	if len(matches) == 1 {
		result += "/"
	}
	return result
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

func (m cmdbarModel) View(width int) string {
	if m.active {
		return styleCmdPrefix.Render(":") + m.input
	}
	return styleCmdPrefix.Render("> ")
}
