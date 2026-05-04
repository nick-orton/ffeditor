package main

import "github.com/charmbracelet/lipgloss"

var (
	styleHeader     = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("62"))
	styleDir        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleAudio      = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleCursor     = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	styleSelected   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleStatusOk   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleStatusErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleTagLabel   = lipgloss.NewStyle().Width(10).Align(lipgloss.Right)
	styleTagFocused  = lipgloss.NewStyle().Underline(true)
	styleTagDisabled = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleCmdPrefix  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSymlink    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleTagInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleNoTags     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)
