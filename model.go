package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBrowse mode = iota
	modeCommand
	modeTag
)

// Phase 1 stubs — replaced in Phase 2 and Phase 4 respectively.
type cmdbarModel struct {
	input  string
	active bool
}

type taggerModel struct {
	width int
}

type model struct {
	mode            mode
	width, height   int
	browser         browserModel
	tagger          taggerModel
	cmdbar          cmdbarModel
	statusMsg       string
	statusIsError   bool
	ffmpegAvailable bool
}

func newModel(dir string, ffmpegAvailable bool) model {
	return model{
		mode:            modeBrowse,
		browser:         newBrowserModel(dir),
		ffmpegAvailable: ffmpegAvailable,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.browser.height = m.height - 4

	case dirChangedMsg:
		// Browser dir already updated; hook point for future phases.

	case tea.KeyMsg:
		if msg.String() == "q" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeBrowse:
			var cmd tea.Cmd
			m.browser, cmd = m.browser.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m model) View() string {
	headerText := fmt.Sprintf(" FFEditor    %s ", m.browser.dir)
	header := styleHeader.Width(m.width).Render(headerText)

	browserHeight := m.height - 4
	if browserHeight < 0 {
		browserHeight = 0
	}
	browserView := m.browser.View(m.width, browserHeight)

	var statusLine string
	if m.statusIsError {
		statusLine = styleStatusErr.Render(m.statusMsg)
	} else if m.statusMsg != "" {
		statusLine = styleStatusOk.Render(m.statusMsg)
	}

	cmdBar := styleCmdPrefix.Render("> ")

	return lipgloss.JoinVertical(lipgloss.Left, header, browserView, statusLine, cmdBar)
}
