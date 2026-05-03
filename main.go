package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error getting working directory:", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		dir, err = filepath.Abs(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid directory:", err)
			os.Exit(1)
		}
	}

	_, ffmpegErr := exec.LookPath("ffmpeg")
	ffmpegAvailable := ffmpegErr == nil

	m := newModel(dir, ffmpegAvailable)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
