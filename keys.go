package main

import tea "github.com/charmbracelet/bubbletea"

// handleKeyMsg dispatches a key event to the handler for the current mode.
func handleKeyMsg(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeBrowse:
		return updateBrowseMode(m, msg)
	case modeTag:
		return updateTagMode(m, msg)
	case modeTagSaving, modeTagSearching:
		return m, nil
	case modeHelp:
		return updateHelpMode(m, msg)
	case modeCommand:
		return updateCommandMode(m, msg)
	}
	return m, nil
}

func updateBrowseMode(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.convertCancel != nil {
			m.convertCancel()
			m.convertCancelled = true
			m.statusMsg = "Conversion cancelled"
			m.statusIsError = false
		}
		return m, nil
	case "q":
		return m, tea.Quit
	case ":":
		m.mode = modeCommand
		m.cmdbar.active = true
		m.cmdbar.input = ""
		return m, nil
	case "e":
		return dispatchCommand(m, "edit", nil)
	case "c":
		return dispatchCommand(m, "convert", nil)
	case "?":
		m.mode = modeHelp
		return m, nil
	}
	var cmd tea.Cmd
	m.browser, cmd = m.browser.Update(msg)
	return m, cmd
}

func updateTagMode(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+t":
		if len(m.tagger.files) == 1 {
			m.mode = modeTagSearching
			m.spinnerFrame = 0
			return m, tea.Batch(claudeGuessTagsCmd(m.tagger.files[0]), spinnerTick())
		}
	case "ctrl+s":
		m.mode = modeTagSaving
		m.spinnerFrame = 0
		return m, tea.Batch(m.tagger.saveTags(), spinnerTick())
	}
	var cmd tea.Cmd
	m.tagger, cmd = m.tagger.Update(msg)
	return m, cmd
}

func updateHelpMode(m model, _ tea.KeyMsg) (model, tea.Cmd) {
	m.mode = modeBrowse
	return m, nil
}

func updateCommandMode(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		cmd, args := parseCommand(m.cmdbar.input)
		m.cmdbar.input = ""
		m.cmdbar.active = false
		m.mode = modeBrowse
		return dispatchCommand(m, cmd, args)
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
