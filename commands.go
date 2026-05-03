package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type cmdbarModel struct {
	input  string
	active bool
}

func parseCommand(input string) (cmd string, args []string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func (m cmdbarModel) Update(msg tea.Msg) (cmdbarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.input = ""
			m.active = false
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.Runes) == 1 {
				m.input += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m cmdbarModel) View(width int) string {
	if m.active {
		return styleCmdPrefix.Render(":") + m.input
	}
	return styleCmdPrefix.Render("> ")
}
