package main

import (
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tagField struct {
	label    string
	value    string
	original string
}

type taggerModel struct {
	files      []string
	fields     []tagField
	focusIndex int
	width      int
	tokens     []string // tokens parsed from filename(s)
	tabStem    string   // field value before the word being completed
	tabPrefix  string   // the word prefix when the tab cycle started
	tabMatches []string // candidates for current cycle
	tabIndex   int      // next index within tabMatches
}

type tagSavedMsg struct{}
type tagBulkSavedMsg struct{ count int }
type tagCancelledMsg struct{}
type tagErrMsg struct{ err error }

func newTaggerModel(files []string) (taggerModel, error) {
	fields := []tagField{
		{label: "Title"},
		{label: "Artist"},
		{label: "Album"},
		{label: "Year"},
		{label: "Track"},
		{label: "Genre"},
	}

	if len(files) == 1 {
		data, err := readTags(files[0])
		if err != nil {
			return taggerModel{}, err
		}
		vals := [6]string{data.Title, data.Artist, data.Album, data.Year, data.Track, data.Genre}
		for i, v := range vals {
			fields[i].value = v
			fields[i].original = v
		}
	} else {
		// Bulk mode: pre-fill fields where all files share the same value.
		allVals := make([][]string, len(files))
		for i, file := range files {
			data, err := readTags(file)
			vals := make([]string, 6)
			if err == nil {
				vals[0] = data.Title
				vals[1] = data.Artist
				vals[2] = data.Album
				vals[3] = data.Year
				vals[4] = data.Track
				vals[5] = data.Genre
			}
			allVals[i] = vals
		}
		for fi := range fields {
			seed := allVals[0][fi]
			if seed == "" {
				continue
			}
			agree := true
			for _, vals := range allVals[1:] {
				if vals[fi] != seed {
					agree = false
					break
				}
			}
			if agree {
				fields[fi].value = seed
				fields[fi].original = seed
			}
		}
	}

	focusIndex := 0
	if len(files) > 1 {
		focusIndex = 1 // Title is not editable in bulk mode
	}

	return taggerModel{
		files:      files,
		fields:     fields,
		focusIndex: focusIndex,
		tokens:     tokenizeFilenames(files),
	}, nil
}

// tokenizeFilenames splits filenames on non-alphanumeric characters and returns
// a deduplicated list of tokens, preserving order of first appearance.
func tokenizeFilenames(files []string) []string {
	seen := make(map[string]bool)
	var tokens []string
	for _, file := range files {
		name := filepath.Base(file)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		parts := strings.FieldsFunc(name, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		for _, p := range parts {
			key := strings.ToLower(p)
			if !seen[key] {
				seen[key] = true
				tokens = append(tokens, p)
			}
		}
	}
	return tokens
}

func (m taggerModel) Update(msg tea.Msg) (taggerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m = m.handleTab()
			return m, nil
		case "down":
			next := (m.focusIndex + 1) % 6
			if len(m.files) > 1 && next == 0 {
				next = 1
			}
			m.focusIndex = next
			m.tabMatches = nil
		case "shift+tab", "up":
			next := (m.focusIndex + 5) % 6
			if len(m.files) > 1 && next == 0 {
				next = 5
			}
			m.focusIndex = next
			m.tabMatches = nil
		case "esc":
			return m, func() tea.Msg { return tagCancelledMsg{} }
		case "backspace":
			m.tabMatches = nil
			v := m.fields[m.focusIndex].value
			if len(v) > 0 {
				m.fields[m.focusIndex].value = v[:len(v)-1]
			}
		default:
			if len(msg.Runes) == 1 && unicode.IsPrint(msg.Runes[0]) {
				m.tabMatches = nil
				m.fields[m.focusIndex].value += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m taggerModel) handleTab() taggerModel {
	if len(m.tokens) == 0 {
		return m
	}
	if m.tabMatches == nil {
		// Split field value into the part before the current word and the word itself.
		current := m.fields[m.focusIndex].value
		lastSpace := strings.LastIndexAny(current, " \t")
		if lastSpace >= 0 {
			m.tabStem = current[:lastSpace+1]
			m.tabPrefix = current[lastSpace+1:]
		} else {
			m.tabStem = ""
			m.tabPrefix = current
		}
		prefix := strings.ToLower(m.tabPrefix)
		for _, tok := range m.tokens {
			if strings.HasPrefix(strings.ToLower(tok), prefix) {
				m.tabMatches = append(m.tabMatches, tok)
			}
		}
		if len(m.tabMatches) == 0 {
			return m
		}
		m.tabIndex = 0
	}
	m.fields[m.focusIndex].value = m.tabStem + m.tabMatches[m.tabIndex]
	m.tabIndex = (m.tabIndex + 1) % len(m.tabMatches)
	return m
}

func (m taggerModel) saveTags() tea.Cmd {
	files := m.files
	fields := make([]tagField, len(m.fields))
	copy(fields, m.fields)

	return func() tea.Msg {
		if len(files) == 1 {
			data := tagData{
				Title:  fields[0].value,
				Artist: fields[1].value,
				Album:  fields[2].value,
				Year:   fields[3].value,
				Track:  fields[4].value,
				Genre:  fields[5].value,
			}
			var mask [6]bool
			for i, f := range fields {
				mask[i] = f.value != f.original
			}
			if err := writeTags(files[0], data, mask); err != nil {
				return tagErrMsg{err}
			}
			return tagSavedMsg{}
		}

		// Bulk tagging: only write non-empty fields.
		count := 0
		for _, file := range files {
			data := tagData{
				Title:  fields[0].value,
				Artist: fields[1].value,
				Album:  fields[2].value,
				Year:   fields[3].value,
				Track:  fields[4].value,
				Genre:  fields[5].value,
			}
			var mask [6]bool
			anySet := false
			for i, f := range fields {
				if f.value != "" {
					mask[i] = true
					anySet = true
				}
			}
			if !anySet {
				continue
			}
			if err := writeTags(file, data, mask); err != nil {
				continue
			}
			count++
		}
		return tagBulkSavedMsg{count}
	}
}

func (m taggerModel) View(width, height int) string {
	boxWidth := width - 4
	if boxWidth < 44 {
		boxWidth = 44
	}

	// Files box.
	var fileLines []string
	for _, f := range m.files {
		fileLines = append(fileLines, filepath.Base(f))
	}
	filesBox := titledBox("Files", strings.Join(fileLines, "\n"), boxWidth)

	// Tags box.
	var fieldLines []string
	for i, f := range m.fields {
		label := styleTagLabel.Render(f.label + ":")
		var val string
		if len(m.files) > 1 && i == 0 {
			val = styleTagDisabled.Render(f.value)
		} else if i == m.focusIndex {
			val = styleTagFocused.Render(f.value) + "▌"
		} else {
			val = f.value
		}
		fieldLines = append(fieldLines, label+" "+val)
	}
	tagsBox := titledBox("Tags", strings.Join(fieldLines, "\n"), boxWidth)

	hint := "  Up/Down: navigate   Tab: complete   Ctrl+S: save   Esc: cancel"
	if len(m.files) == 1 {
		hint = "  Up/Down: navigate   Tab: complete   Ctrl+T: smart tags   Ctrl+S: save   Esc: cancel"
	}
	content := strings.Join([]string{filesBox, "", tagsBox, "", hint}, "\n")

	// Vertically center.
	contentHeight := strings.Count(content, "\n") + 1
	topPad := (height - contentHeight) / 2
	if topPad <= 0 {
		return content
	}
	return strings.Repeat("\n", topPad) + content
}

// titledBox draws a rounded box with a title embedded in the top border.
func titledBox(title, content string, width int) string {
	b := lipgloss.RoundedBorder()
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	inner := width - 2 // chars between the two corner chars

	// Top border: ╭─ Title ──...──╮
	titleSection := " " + title + " "
	dashCount := inner - 1 - len(titleSection)
	if dashCount < 0 {
		dashCount = 0
	}
	top := b.TopLeft + b.Top + titleSection + strings.Repeat(b.Top, dashCount) + b.TopRight
	bottom := b.BottomLeft + strings.Repeat(b.Bottom, inner) + b.BottomRight

	var lines []string
	lines = append(lines, borderStyle.Render(top))
	for _, line := range strings.Split(content, "\n") {
		padWidth := inner - 2 - lipgloss.Width(line)
		if padWidth < 0 {
			padWidth = 0
		}
		lines = append(lines, borderStyle.Render(b.Left)+" "+line+strings.Repeat(" ", padWidth)+" "+borderStyle.Render(b.Right))
	}
	lines = append(lines, borderStyle.Render(bottom))

	return strings.Join(lines, "\n")
}
