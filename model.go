package main

import (
	"context"
	"fmt"
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
	modeSmartTagging
	modeFilter
)

type spinnerTickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

type model struct {
	mode          mode
	width, height int

	browser browserModel
	tagger  taggerModel
	cmdbar  cmdbarModel

	statusMsg     string
	statusIsError bool

	spinnerFrame    int
	ffmpegAvailable bool

	// convert pipeline state
	convertQueue     []string
	convertIndex     int
	convertDone      int
	convertSkipped   int
	convertErrors    int
	convertCtx       context.Context
	convertCancel    context.CancelFunc
	convertCancelled bool
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

func (m model) withError(msg string) model {
	m.statusMsg = msg
	m.statusIsError = true
	return m
}

func (m model) withMessage(msg string) model {
	m.statusMsg = msg
	m.statusIsError = false
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m2, cmd, ok := handleConvertMsg(m, msg); ok {
		return m2, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.browser.height = m.height - 4
		m.tagger.width = m.width

	case execTagMsg:
		tagger, err := newTaggerModel(msg.files)
		if err != nil {
			return m.withError("Error opening tag: "+err.Error()), nil
		}
		tagger.width = m.width
		m.tagger = tagger
		m.mode = modeTag
		return m, nil

	case spinnerTickMsg:
		if m.mode == modeTagSaving || m.mode == modeTagSearching || m.mode == modeSmartTagging {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, spinnerTick()
		}
		return m, nil

	case tagSavedMsg:
		m.mode = modeBrowse
		m = m.withMessage("Tags saved")
		m.browser.tagCache = loadTagCache(m.browser.entries, m.browser.dir)
		return m, nil

	case tagBulkSavedMsg:
		m.mode = modeBrowse
		m = m.withMessage(fmt.Sprintf("Tags updated (%d files)", msg.count))
		m.browser.tagCache = loadTagCache(m.browser.entries, m.browser.dir)
		return m, nil

	case tagCancelledMsg:
		m.mode = modeBrowse
		return m.withMessage(""), nil

	case tagErrMsg:
		m.mode = modeBrowse
		return m.withError("Tag error: "+msg.err.Error()), nil

	case tagSearchResultMsg:
		m.mode = modeTag
		if m.tagger.fields[FieldTitle].value == "" {
			m.tagger.fields[FieldTitle].value = msg.title
		}
		if m.tagger.fields[FieldArtist].value == "" {
			m.tagger.fields[FieldArtist].value = msg.artist
		}
		if m.tagger.fields[FieldYear].value == "" {
			m.tagger.fields[FieldYear].value = msg.year
		}
		return m, nil

	case tagSearchErrMsg:
		m.mode = modeTag
		return m.withError("Smart tag error: "+msg.err.Error()), nil

	case smartTagDoneMsg:
		m.mode = modeBrowse
		m = m.withMessage(fmt.Sprintf("Smart tags applied (%d files)", msg.count))
		m.browser.tagCache = loadTagCache(m.browser.entries, m.browser.dir)
		return m, nil

	case smartTagErrMsg:
		m.mode = modeBrowse
		return m.withError("Smart tag error: "+msg.err.Error()), nil

	case dirReadErrMsg:
		return m.withError("Cannot read directory: "+msg.err.Error()), nil

	case dirChangedMsg:
		// Browser dir already updated; hook point for future phases.

	case tea.KeyMsg:
		return handleKeyMsg(m, msg)
	}

	return m, nil
}

func dispatchCommand(m model, cmd string, args []string) (model, tea.Cmd) {
	if h, ok := commandHandlers[cmd]; ok {
		return h(m, args)
	}
	if cmd != "" {
		m = m.withError("Unknown command: " + cmd)
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
	} else if m.mode == modeSmartTagging {
		statusLine = styleStatusOk.Render(spinnerFrames[m.spinnerFrame] + " Applying smart tags...")
	} else if m.statusIsError {
		statusLine = styleStatusErr.Render(m.statusMsg)
	} else if m.statusMsg != "" {
		statusLine = styleStatusOk.Render(m.statusMsg)
	}

	var cmdBar string
	switch m.mode {
	case modeFilter:
		cmdBar = styleCmdPrefix.Render("/") + m.browser.filterInput
	default:
		if m.browser.filterInput != "" {
			cmdBar = styleTagInfo.Render("filter: " + m.browser.filterInput)
		} else {
			cmdBar = m.cmdbar.View(m.width)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, browserView, statusLine, cmdBar)
}
