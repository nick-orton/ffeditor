package main

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// knownCommands is the sorted list of valid command names used for tab completion.
var knownCommands = []string{"cd", "convert", "q", "tag"}

type cmdbarModel struct {
	input      string
	active     bool
	tabPrefix  string   // input prefix when the current tab cycle started
	tabMatches []string // nil when no active tab cycle
	tabIndex   int
}

type execConvertMsg struct{ files []string }
type execTagMsg struct{ files []string }

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
			m.tabMatches = nil
			m.tabIndex = 0
		case "backspace":
			m.tabMatches = nil
			m.tabIndex = 0
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			m.tabMatches = nil
			m.tabIndex = 0
			if len(msg.Runes) == 1 {
				m.input += string(msg.Runes)
			}
		}
	}
	return m, nil
}

// handleTab performs tab completion. If the input is a bare word (no spaces),
// it cycles through command names that start with that prefix. If the input
// starts with "cd ", it completes the path argument.
func (m cmdbarModel) handleTab(browserDir string) cmdbarModel {
	trimmed := strings.TrimSpace(m.input)

	if !strings.Contains(trimmed, " ") {
		// Command-name completion / cycling.
		if m.tabMatches == nil {
			m.tabPrefix = trimmed
			m.tabMatches = commandsStartingWith(trimmed)
			m.tabIndex = 0
		}
		if len(m.tabMatches) > 0 {
			m.input = m.tabMatches[m.tabIndex]
			m.tabIndex = (m.tabIndex + 1) % len(m.tabMatches)
		}
		return m
	}

	// Path completion for cd arguments (no cycling — longest-common-prefix).
	m.tabMatches = nil
	m.tabIndex = 0
	m.input = tabCompletePath(m.input, browserDir)
	return m
}

func commandsStartingWith(prefix string) []string {
	var result []string
	for _, cmd := range knownCommands {
		if strings.HasPrefix(cmd, prefix) {
			result = append(result, cmd)
		}
	}
	return result
}

// tabCompletePath attempts directory tab-completion for the "cd" command.
// browserDir is used as the base for resolving relative paths.
func tabCompletePath(input, browserDir string) string {
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
