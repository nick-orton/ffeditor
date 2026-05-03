package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dirChangedMsg struct{ path string }
type dirReadErrMsg struct {
	path string
	err  error
}

var audioExts = map[string]bool{
	".mp3": true, ".opus": true, ".m4a": true,
	".flac": true, ".ogg": true,
}

func isAudio(name string) bool {
	return audioExts[strings.ToLower(filepath.Ext(name))]
}

type browserModel struct {
	dir      string
	entries  []os.DirEntry
	cursor   int
	offset   int
	selected map[int]bool
	height   int
}

func newBrowserModel(dir string) browserModel {
	m := browserModel{
		dir:      dir,
		selected: make(map[int]bool),
	}
	entries, err := os.ReadDir(dir)
	if err == nil {
		sortEntries(entries)
		m.entries = entries
	}
	return m
}

func sortEntries(entries []os.DirEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].IsDir() && !entries[j].IsDir()
	})
}

func (m browserModel) changeDir(dir string) (browserModel, tea.Cmd) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.dir = dir
		m.entries = nil
		m.cursor = 0
		m.offset = 0
		m.selected = make(map[int]bool)
		return m, func() tea.Msg { return dirReadErrMsg{dir, err} }
	}
	sortEntries(entries)
	m.dir = dir
	m.entries = entries
	m.cursor = 0
	m.offset = 0
	m.selected = make(map[int]bool)
	return m, func() tea.Msg { return dirChangedMsg{dir} }
}

func (m browserModel) scrollUp() browserModel {
	if m.cursor > 0 {
		m.cursor--
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
	}
	return m
}

func (m browserModel) scrollDown() browserModel {
	if m.cursor < len(m.entries)-1 {
		m.cursor++
		if m.height > 0 && m.cursor >= m.offset+m.height {
			m.offset = m.cursor - m.height + 1
		}
	}
	return m
}

func (m browserModel) Update(msg tea.Msg) (browserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m = m.scrollDown()
		case "k", "up":
			m = m.scrollUp()
		case "enter":
			if len(m.entries) > 0 && m.entries[m.cursor].IsDir() {
				newDir := filepath.Join(m.dir, m.entries[m.cursor].Name())
				return m.changeDir(newDir)
			}
		case "h":
			parent := filepath.Dir(m.dir)
			if parent != m.dir {
				return m.changeDir(parent)
			}
		case "l":
			if len(m.entries) > 0 && m.entries[m.cursor].IsDir() {
				newDir := filepath.Join(m.dir, m.entries[m.cursor].Name())
				return m.changeDir(newDir)
			}
		case " ":
			if len(m.entries) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
				if !m.selected[m.cursor] {
					delete(m.selected, m.cursor)
				}
				m = m.scrollDown()
			}
		}
	}
	return m, nil
}

func (m browserModel) selectedEntries() []os.DirEntry {
	if len(m.selected) == 0 {
		if len(m.entries) == 0 {
			return nil
		}
		return []os.DirEntry{m.entries[m.cursor]}
	}
	var result []os.DirEntry
	for i, entry := range m.entries {
		if m.selected[i] {
			result = append(result, entry)
		}
	}
	return result
}

func (m browserModel) View(width, height int) string {
	if height <= 0 {
		return ""
	}

	var lines []string

	end := m.offset + height
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		entry := m.entries[i]
		name := entry.Name()

		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}

		var styledName string
		switch {
		case m.selected[i]:
			displayName := name
			if entry.IsDir() {
				displayName = name + "/"
			}
			styledName = styleSelected.Render(displayName)
		case entry.IsDir():
			styledName = styleDir.Render(name + "/")
		case isAudio(name):
			styledName = styleAudio.Render(name)
		case strings.HasPrefix(name, "."):
			styledName = lipgloss.NewStyle().Faint(true).Render(name)
		default:
			styledName = name
		}

		line := prefix + styledName
		if i == m.cursor {
			line = styleCursor.Width(width).Render(line)
		}

		lines = append(lines, line)
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
