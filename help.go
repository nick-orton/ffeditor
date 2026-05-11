package main

import "strings"

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
		"    ctrl+a      select all entries",
		"    i           toggle hidden files",
		"",
		"  Filter",
		"    /           start filtering entries",
		"    enter       apply filter",
		"    esc         clear filter / cancel",
		"",
		"  Commands",
		"    e           edit tags",
		"    c           convert file(s)",
		"    ctrl+t      fill missing tags (smart tags)",
		"    :cd <dir>   change directory",
		"    q           quit",
		"",
		"  Other",
		"    ?           show this help",
		"    esc         close help",
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}
