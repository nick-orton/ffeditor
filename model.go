package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBrowse mode = iota
	modeCommand
	modeTag
)

// Phase 4 stub — replaced when tagger.go is created.
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
		switch m.mode {
		case modeBrowse:
			if msg.String() == "q" {
				return m, tea.Quit
			}
			if msg.String() == ":" {
				m.mode = modeCommand
				m.cmdbar.active = true
				m.cmdbar.input = ""
				return m, nil
			}
			var cmd tea.Cmd
			m.browser, cmd = m.browser.Update(msg)
			return m, cmd

		case modeCommand:
			if msg.String() == "enter" {
				cmd, args := parseCommand(m.cmdbar.input)
				m.cmdbar.input = ""
				m.cmdbar.active = false
				m.mode = modeBrowse
				var teaCmd tea.Cmd
				m, teaCmd = dispatchCommand(m, cmd, args)
				return m, teaCmd
			}
			var cmd tea.Cmd
			m.cmdbar, cmd = m.cmdbar.Update(msg)
			if !m.cmdbar.active {
				m.mode = modeBrowse
			}
			return m, cmd
		}
	}

	return m, nil
}

func dispatchCommand(m model, cmd string, args []string) (model, tea.Cmd) {
	switch cmd {
	case "":
		return m, nil
	case "q":
		return m, tea.Quit
	case "cd":
		var target string
		if len(args) == 0 {
			home, err := os.UserHomeDir()
			if err != nil {
				m.statusMsg = "Could not determine home directory"
				m.statusIsError = true
				return m, nil
			}
			target = home
		} else {
			arg := args[0]
			if filepath.IsAbs(arg) {
				target = arg
			} else {
				target = filepath.Join(m.browser.dir, arg)
			}
			var err error
			target, err = filepath.Abs(target)
			if err != nil {
				m.statusMsg = "Not a directory: " + args[0]
				m.statusIsError = true
				return m, nil
			}
		}
		info, err := os.Stat(target)
		if err != nil || !info.IsDir() {
			m.statusMsg = "Not a directory: " + target
			m.statusIsError = true
			return m, nil
		}
		var teaCmd tea.Cmd
		m.browser, teaCmd = m.browser.changeDir(target)
		m.statusMsg = ""
		m.statusIsError = false
		return m, teaCmd
	default:
		m.statusMsg = "Unknown command: " + cmd
		m.statusIsError = true
		return m, nil
	}
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

	cmdBar := m.cmdbar.View(m.width)

	return lipgloss.JoinVertical(lipgloss.Left, header, browserView, statusLine, cmdBar)
}
