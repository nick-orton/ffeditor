package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	id3 "github.com/bogem/id3v2/v2"
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
		tag, err := id3.Open(files[0], id3.Options{Parse: true})
		if err != nil {
			return taggerModel{}, err
		}
		defer tag.Close()

		fields[0].value = tag.Title()
		fields[0].original = tag.Title()
		fields[1].value = tag.Artist()
		fields[1].original = tag.Artist()
		fields[2].value = tag.Album()
		fields[2].original = tag.Album()
		fields[3].value = tag.Year()
		fields[3].original = tag.Year()

		if frame := tag.GetLastFrame("TRCK"); frame != nil {
			if tf, ok := frame.(id3.TextFrame); ok {
				fields[4].value = tf.Text
				fields[4].original = tf.Text
			}
		}

		fields[5].value = tag.Genre()
		fields[5].original = tag.Genre()
	}

	return taggerModel{
		files:  files,
		fields: fields,
		tokens: tokenizeFilenames(files),
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
			m.focusIndex = (m.focusIndex + 1) % 6
			m.tabMatches = nil
		case "shift+tab", "up":
			m.focusIndex = (m.focusIndex + 5) % 6
			m.tabMatches = nil
		case "ctrl+s":
			return m, m.saveTags()
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
			tag, err := id3.Open(files[0], id3.Options{Parse: true})
			if err != nil {
				return tagErrMsg{err}
			}
			defer tag.Close()

			for i, f := range fields {
				if f.value == f.original {
					continue
				}
				switch i {
				case 0:
					tag.SetTitle(f.value)
				case 1:
					tag.SetArtist(f.value)
				case 2:
					tag.SetAlbum(f.value)
				case 3:
					tag.SetYear(f.value)
				case 4:
					tag.DeleteFrames("TRCK")
					if f.value != "" {
						tag.AddTextFrame("TRCK", id3.EncodingUTF8, f.value)
					}
				case 5:
					tag.SetGenre(f.value)
				}
			}

			if err := tag.Save(); err != nil {
				return tagErrMsg{err}
			}
			return tagSavedMsg{}
		}

		// Bulk tagging: only write non-empty fields.
		count := 0
		for _, file := range files {
			tag, err := id3.Open(file, id3.Options{Parse: true})
			if err != nil {
				continue
			}
			changed := false
			for i, f := range fields {
				if f.value == "" {
					continue
				}
				changed = true
				switch i {
				case 0:
					tag.SetTitle(f.value)
				case 1:
					tag.SetArtist(f.value)
				case 2:
					tag.SetAlbum(f.value)
				case 3:
					tag.SetYear(f.value)
				case 4:
					tag.DeleteFrames("TRCK")
					tag.AddTextFrame("TRCK", id3.EncodingUTF8, f.value)
				case 5:
					tag.SetGenre(f.value)
				}
			}
			if changed {
				tag.Save()
				count++
			}
			tag.Close()
		}
		return tagBulkSavedMsg{count}
	}
}

func (m taggerModel) View(width, height int) string {
	lines := make([]string, 0, height)

	// Vertically center: title + blank + 6 fields + blank + hint = 10 lines.
	const formHeight = 10
	topPad := (height - formHeight) / 2
	if topPad < 0 {
		topPad = 0
	}
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}

	// Filename header.
	var title string
	if len(m.files) == 1 {
		title = "  " + filepath.Base(m.files[0])
	} else {
		title = fmt.Sprintf("  %d files", len(m.files))
	}
	lines = append(lines, title)
	lines = append(lines, "")

	for i, f := range m.fields {
		label := styleTagLabel.Render(f.label + ":")
		var val string
		if i == m.focusIndex {
			val = styleTagFocused.Render(f.value) + "▌"
		} else {
			val = f.value
		}
		lines = append(lines, label+" "+val)
	}

	lines = append(lines, "")
	lines = append(lines, "  Up / Down: navigate   Tab: complete   Ctrl+S: save   Esc: cancel")

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
