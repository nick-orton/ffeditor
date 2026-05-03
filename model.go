package main

import (
	"context"
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
	convertQueue      []string
	convertIndex      int
	convertDone       int
	convertSkipped    int
	convertErrors     int
	convertCtx        context.Context
	convertCancel     context.CancelFunc
	convertCancelled  bool
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

	case execConvertMsg:
		if !m.ffmpegAvailable {
			m.statusMsg = "ffmpeg not available"
			m.statusIsError = true
			return m, nil
		}
		if len(msg.files) == 0 {
			m.statusMsg = "No convertible files selected"
			m.statusIsError = true
			return m, nil
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
		return m, convertFile(m.convertCtx, m.convertQueue[0], m.browser.dir)

	case convertDoneMsg:
		m.convertDone++
		var teaCmd tea.Cmd
		m, teaCmd = nextConvert(m)
		return m, teaCmd

	case convertSkippedMsg:
		m.convertSkipped++
		var teaCmd tea.Cmd
		m, teaCmd = nextConvert(m)
		return m, teaCmd

	case convertErrMsg:
		m.convertErrors++
		var teaCmd tea.Cmd
		m, teaCmd = nextConvert(m)
		return m, teaCmd

	case dirChangedMsg:
		// Browser dir already updated; hook point for future phases.

	case tea.KeyMsg:
		switch m.mode {
		case modeBrowse:
			if msg.String() == "ctrl+c" {
				if m.convertCancel != nil {
					m.convertCancel()
					m.convertCancelled = true
					m.statusMsg = "Conversion cancelled"
					m.statusIsError = false
					return m, nil
				}
				return m, nil
			}
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
			switch msg.String() {
			case "enter":
				cmd, args := parseCommand(m.cmdbar.input)
				m.cmdbar.input = ""
				m.cmdbar.active = false
				m.mode = modeBrowse
				var teaCmd tea.Cmd
				m, teaCmd = dispatchCommand(m, cmd, args)
				return m, teaCmd
			case "tab":
				m.cmdbar.input = tabComplete(m.cmdbar.input, m.browser.dir)
				return m, nil
			default:
				var cmd tea.Cmd
				m.cmdbar, cmd = m.cmdbar.Update(msg)
				if !m.cmdbar.active {
					m.mode = modeBrowse
				}
				return m, cmd
			}
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
			arg := expandTilde(args[0])
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
	case "convert":
		if !m.ffmpegAvailable {
			m.statusMsg = "ffmpeg not available — conversion disabled"
			m.statusIsError = true
			return m, nil
		}
		entries := m.browser.selectedEntries()
		files := buildConvertList(entries, m.browser.dir)
		if len(files) == 0 {
			m.statusMsg = "No convertible files selected (.opus or .m4a)"
			m.statusIsError = true
			return m, nil
		}
		return m, func() tea.Msg { return execConvertMsg{files} }
	default:
		m.statusMsg = "Unknown command: " + cmd
		m.statusIsError = true
		return m, nil
	}
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
		return m, convertFile(m.convertCtx, m.convertQueue[m.convertIndex], m.browser.dir)
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
