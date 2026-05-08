package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeBrowse mode = iota
	modeCommand
	modeTag
	modeTagSaving
	modeTagSearching
	modeHelp
)

type spinnerTickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

type model struct {
	mode            mode
	width, height   int
	browser         browserModel
	tagger          taggerModel
	cmdbar          cmdbarModel
	statusMsg       string
	statusIsError   bool
	spinnerFrame    int
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
	m := model{
		mode:            modeBrowse,
		browser:         newBrowserModel(dir),
		ffmpegAvailable: ffmpegAvailable,
	}
	if !ffmpegAvailable {
		m.statusMsg = "ffmpeg not found — conversion disabled"
	}
	return m
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
		m.tagger.width = m.width

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
		return m, convertFile(m.convertCtx, m.convertQueue[0])

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

	case execTagMsg:
		tagger, err := newTaggerModel(msg.files)
		if err != nil {
			m.statusMsg = "Error opening tag: " + err.Error()
			m.statusIsError = true
			return m, nil
		}
		tagger.width = m.width
		m.tagger = tagger
		m.mode = modeTag
		return m, nil

	case spinnerTickMsg:
		if m.mode == modeTagSaving || m.mode == modeTagSearching {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, spinnerTick()
		}
		return m, nil

	case tagSavedMsg:
		m.mode = modeBrowse
		m.statusMsg = "Tags saved"
		m.statusIsError = false
		m.browser.tagCache = loadTagCache(m.browser.entries, m.browser.dir)
		return m, nil

	case tagBulkSavedMsg:
		m.mode = modeBrowse
		m.statusMsg = fmt.Sprintf("Tags updated (%d files)", msg.count)
		m.statusIsError = false
		m.browser.tagCache = loadTagCache(m.browser.entries, m.browser.dir)
		return m, nil

	case tagCancelledMsg:
		m.mode = modeBrowse
		m.statusMsg = ""
		m.statusIsError = false
		return m, nil

	case tagErrMsg:
		m.mode = modeBrowse
		m.statusMsg = "Tag error: " + msg.err.Error()
		m.statusIsError = true
		return m, nil

	case tagSearchResultMsg:
		m.mode = modeTag
		if m.tagger.fields[0].value == "" {
			m.tagger.fields[0].value = msg.title
		}
		if m.tagger.fields[1].value == "" {
			m.tagger.fields[1].value = msg.artist
		}
		if m.tagger.fields[3].value == "" {
			m.tagger.fields[3].value = msg.year
		}
		return m, nil

	case tagSearchErrMsg:
		m.mode = modeTag
		m.statusMsg = "Smart tag error: " + msg.err.Error()
		m.statusIsError = true
		return m, nil

	case dirReadErrMsg:
		m.statusMsg = "Cannot read directory: " + msg.err.Error()
		m.statusIsError = true
		return m, nil

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
			if msg.String() == "e" {
				var teaCmd tea.Cmd
				m, teaCmd = dispatchCommand(m, "edit", nil)
				return m, teaCmd
			}
			if msg.String() == "c" {
				var teaCmd tea.Cmd
				m, teaCmd = dispatchCommand(m, "convert", nil)
				return m, teaCmd
			}
			if msg.String() == "?" {
				m.mode = modeHelp
				return m, nil
			}
			var cmd tea.Cmd
			m.browser, cmd = m.browser.Update(msg)
			return m, cmd

		case modeTag:
			if msg.String() == "ctrl+t" && len(m.tagger.files) == 1 {
				m.mode = modeTagSearching
				m.spinnerFrame = 0
				return m, tea.Batch(claudeGuessTagsCmd(m.tagger.files[0]), spinnerTick())
			}
			if msg.String() == "ctrl+s" {
				m.mode = modeTagSaving
				m.spinnerFrame = 0
				return m, tea.Batch(m.tagger.saveTags(), spinnerTick())
			}
			var cmd tea.Cmd
			m.tagger, cmd = m.tagger.Update(msg)
			return m, cmd

		case modeTagSaving, modeTagSearching:
			return m, nil

		case modeHelp:
			m.mode = modeBrowse
			return m, nil

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
				m.cmdbar = m.cmdbar.handleTab(m.browser.dir)
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
	case "tag", "edit":
		entries := m.browser.selectedEntries()
		var mp3s []string
		for _, e := range entries {
			if !e.IsDir() && strings.ToLower(filepath.Ext(e.Name())) == ".mp3" {
				mp3s = append(mp3s, filepath.Join(m.browser.dir, e.Name()))
			}
		}
		if len(mp3s) == 0 {
			m.statusMsg = "No .mp3 files selected"
			m.statusIsError = true
			return m, nil
		}
		return m, func() tea.Msg { return execTagMsg{mp3s} }
	default:
		m.statusMsg = "Unknown command: " + cmd
		m.statusIsError = true
		return m, nil
	}
}

func helpView(width, height int) string {
	lines := []string{
		styleHeader.Width(width).Render(" Keybindings "),
		"",
		"  Navigation",
		"    j / ↓       move down",
		"    k / ↑       move up",
		"    gg          go to first entry",
		"    G           go to last entry",
		"    ctrl+u      page up",
		"    ctrl+d      page down",
		"    l / enter   open directory",
		"    h           go to parent directory",
		"",
		"  Selection",
		"    space       select/deselect entry",
		"    i           toggle hidden files",
		"",
		"  Commands",
		"    e           edit tags",
		"    c           convert file(s)",
		"    :cd <dir>   change directory",
		"    q           quit",
		"",
		"  Other",
		"    ?           show this help",
		"    esc         close help",
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
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

func (m model) View() string {
	headerText := fmt.Sprintf(" FFEditor    %s ", m.browser.dir)
	header := styleHeader.Width(m.width).Render(headerText)

	browserHeight := m.height - 4
	if browserHeight < 0 {
		browserHeight = 0
	}
	var browserView string
	switch m.mode {
	case modeTag, modeTagSaving, modeTagSearching:
		browserView = m.tagger.View(m.width, browserHeight)
	case modeHelp:
		browserView = helpView(m.width, browserHeight)
	default:
		browserView = m.browser.View(m.width, browserHeight)
	}

	var statusLine string
	if m.mode == modeTagSaving {
		statusLine = styleStatusOk.Render(spinnerFrames[m.spinnerFrame] + " Saving...")
	} else if m.mode == modeTagSearching {
		statusLine = styleStatusOk.Render(spinnerFrames[m.spinnerFrame] + " Searching...")
	} else if m.statusIsError {
		statusLine = styleStatusErr.Render(m.statusMsg)
	} else if m.statusMsg != "" {
		statusLine = styleStatusOk.Render(m.statusMsg)
	}

	cmdBar := m.cmdbar.View(m.width)

	return lipgloss.JoinVertical(lipgloss.Left, header, browserView, statusLine, cmdBar)
}
