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

type tagSummary struct {
	artist string
	title  string
}

func (t tagSummary) display() string {
	switch {
	case t.artist != "" && t.title != "":
		return t.artist + " · " + t.title
	case t.artist != "":
		return t.artist
	case t.title != "":
		return t.title
	}
	return ""
}

func loadTagCache(entries []os.DirEntry, dir string) map[string]tagSummary {
	cache := make(map[string]tagSummary)
	for _, e := range entries {
		if !e.IsDir() && isBlessed(e.Name()) {
			cache[e.Name()] = readTagSummary(filepath.Join(dir, e.Name()))
		}
	}
	return cache
}

// truncateRunes shortens s to at most max visible characters, appending "…".
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

type browserModel struct {
	dir        string
	entries    []os.DirEntry
	visible    []os.DirEntry
	filterInput string
	tagCache   map[string]tagSummary
	cursor     int
	offset     int
	selected   map[int]bool
	height     int
	showHidden bool
	pendingG   bool
}

func isSymlinkToDir(entry os.DirEntry, dir string) bool {
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, entry.Name()))
	return err == nil && info.IsDir()
}

func filterEntries(entries []os.DirEntry, showHidden bool) []os.DirEntry {
	var result []os.DirEntry
	for _, e := range entries {
		if showHidden || !strings.HasPrefix(e.Name(), ".") {
			result = append(result, e)
		}
	}
	return result
}

func newBrowserModel(dir string) browserModel {
	m := browserModel{
		dir:      dir,
		selected: make(map[int]bool),
		tagCache: make(map[string]tagSummary),
	}
	entries, err := os.ReadDir(dir)
	if err == nil {
		entries = filterEntries(entries, false)
		sortEntries(entries)
		m.entries = entries
		m.visible = entries
		m.tagCache = loadTagCache(entries, dir)
	}
	return m
}

func sortEntries(entries []os.DirEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		iDir, jDir := entries[i].IsDir(), entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
}

func (m browserModel) changeDir(dir string) (browserModel, tea.Cmd) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.dir = dir
		m.entries = nil
		m.tagCache = make(map[string]tagSummary)
		m.cursor = 0
		m.offset = 0
		m.selected = make(map[int]bool)
		return m, func() tea.Msg { return dirReadErrMsg{dir, err} }
	}
	entries = filterEntries(entries, m.showHidden)
	sortEntries(entries)
	m.dir = dir
	m.entries = entries
	m.visible = entries
	m.filterInput = ""
	m.tagCache = loadTagCache(entries, dir)
	m.cursor = 0
	m.offset = 0
	m.selected = make(map[int]bool)
	return m, func() tea.Msg { return dirChangedMsg{dir} }
}

// scroll moves the cursor by delta rows, clamping to valid bounds and
// adjusting the viewport offset to keep the cursor visible.
func (m browserModel) scroll(delta int) browserModel {
	if len(m.visible) == 0 {
		return m
	}
	m.cursor = max(0, min(m.cursor+delta, len(m.visible)-1))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.height > 0 && m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	return m
}

func (m browserModel) goToFirst() browserModel {
	m.cursor = 0
	m.offset = 0
	return m
}

func (m browserModel) applyFilter(query string) browserModel {
	m.filterInput = query
	m.selected = make(map[int]bool)
	if query == "" {
		m.visible = m.entries
		m.cursor = 0
		m.offset = 0
		return m
	}
	q := strings.ToLower(query)
	var result []os.DirEntry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.Name()), q) {
			result = append(result, e)
		}
	}
	m.visible = result
	m.cursor = 0
	m.offset = 0
	return m
}

func (m browserModel) clearFilter() browserModel {
	var target string
	if len(m.visible) > 0 {
		target = m.visible[m.cursor].Name()
	}
	m.filterInput = ""
	m.visible = m.entries
	m.selected = make(map[int]bool)
	m.cursor = 0
	m.offset = 0
	for i, e := range m.entries {
		if e.Name() == target {
			m.cursor = i
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
			if m.height > 0 && m.cursor >= m.offset+m.height {
				m.offset = m.cursor - m.height + 1
			}
			break
		}
	}
	return m
}

func (m browserModel) goToLast() browserModel {
	if len(m.visible) == 0 {
		return m
	}
	m.cursor = len(m.visible) - 1
	if m.height > 0 && m.cursor >= m.height {
		m.offset = m.cursor - m.height + 1
	}
	return m
}


func (m browserModel) Update(msg tea.Msg) (browserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if m.pendingG {
			m.pendingG = false
			if key == "g" {
				m = m.goToFirst()
				return m, nil
			}
		}
		switch key {
		case "j", "down":
			m = m.scroll(1)
		case "k", "up":
			m = m.scroll(-1)
		case "g":
			m.pendingG = true
		case "G":
			m = m.goToLast()
		case "ctrl+u":
			m = m.scroll(-max(m.height/2, 1))
		case "ctrl+d":
			m = m.scroll(max(m.height/2, 1))
		case "enter":
			if len(m.visible) > 0 {
				entry := m.visible[m.cursor]
				if entry.IsDir() || isSymlinkToDir(entry, m.dir) {
					return m.changeDir(filepath.Join(m.dir, entry.Name()))
				}
			}
		case "h":
			parent := filepath.Dir(m.dir)
			if parent != m.dir {
				childName := filepath.Base(m.dir)
				m, cmd := m.changeDir(parent)
				for i, e := range m.visible {
					if e.Name() == childName {
						m.cursor = i
						if m.height > 0 && m.cursor >= m.offset+m.height {
							m.offset = m.cursor - m.height + 1
						}
						break
					}
				}
				return m, cmd
			}
		case "l":
			if len(m.visible) > 0 {
				entry := m.visible[m.cursor]
				if entry.IsDir() || isSymlinkToDir(entry, m.dir) {
					return m.changeDir(filepath.Join(m.dir, entry.Name()))
				}
			}
		case "i":
			m.showHidden = !m.showHidden
			return m.changeDir(m.dir)
		case " ":
			if len(m.visible) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
				if !m.selected[m.cursor] {
					delete(m.selected, m.cursor)
				}
				m = m.scroll(1)
			}
		case "ctrl+a":
			for i := range m.visible {
				m.selected[i] = true
			}
		}
	}
	return m, nil
}

func (m browserModel) selectedEntries() []os.DirEntry {
	if len(m.selected) == 0 {
		if len(m.visible) == 0 {
			return nil
		}
		return []os.DirEntry{m.visible[m.cursor]}
	}
	var result []os.DirEntry
	for i, entry := range m.visible {
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
	if end > len(m.visible) {
		end = len(m.visible)
	}

	for i := m.offset; i < end; i++ {
		entry := m.visible[i]
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
		case entry.Type()&os.ModeSymlink != 0:
			if isSymlinkToDir(entry, m.dir) {
				styledName = styleSymlink.Render(name + "@/")
			} else {
				styledName = styleSymlink.Render(name + "@")
			}
		case isBlessed(name):
			styledName = styleBlessed.Render(name)
		case isAudio(name):
			styledName = styleAudio.Render(name)
		case strings.HasPrefix(name, "."):
			styledName = lipgloss.NewStyle().Faint(true).Render(name)
		default:
			styledName = name
		}

		nameWidth := lipgloss.Width(prefix + styledName)
		line := prefix + styledName + m.tagColumn(entry.Name(), nameWidth, width)
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

// tagColumn returns the styled tag info portion of a browser row, padded so
// that name+tagColumn fills width. Returns "" when the screen is too narrow or
// the file has no readable tags.
func (m browserModel) tagColumn(name string, nameWidth, totalWidth int) string {
	const minGap = 2
	const minTagWidth = 12
	available := totalWidth - nameWidth - minGap
	if available < minTagWidth {
		return ""
	}
	if !isBlessed(name) {
		return ""
	}
	summary, ok := m.tagCache[name]
	if !ok {
		return ""
	}
	text := summary.display()
	var styled string
	if text == "" {
		styled = styleNoTags.Render("—")
	} else {
		styled = styleTagInfo.Render(truncateRunes(text, available))
	}
	gap := available - lipgloss.Width(styled)
	if gap < minGap {
		gap = minGap
	}
	return strings.Repeat(" ", gap) + styled
}
