package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestReadDirSortOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zdir", "adir"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"bfile.txt", "afile.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sortEntries(entries)

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if !entries[0].IsDir() || !entries[1].IsDir() {
		t.Errorf("expected first two entries to be directories")
	}
	if entries[0].Name() != "adir" || entries[1].Name() != "zdir" {
		t.Errorf("dir order: got %s, %s; want adir, zdir", entries[0].Name(), entries[1].Name())
	}
	if entries[2].IsDir() || entries[3].IsDir() {
		t.Errorf("expected last two entries to be files")
	}
	if entries[2].Name() != "afile.txt" || entries[3].Name() != "bfile.txt" {
		t.Errorf("file order: got %s, %s; want afile.txt, bfile.txt", entries[2].Name(), entries[3].Name())
	}
}

func TestSelectedEntries_NoneSelected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "song.mp3"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	m := newBrowserModel(dir)
	result := m.selectedEntries()

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Name() != "song.mp3" {
		t.Errorf("expected song.mp3, got %s", result[0].Name())
	}
}

func TestSelectedEntries_MultiSelected(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp3", "b.mp3", "c.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	m := newBrowserModel(dir)
	m.selected[0] = true
	m.selected[2] = true

	result := m.selectedEntries()
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Name() != "a.mp3" {
		t.Errorf("expected a.mp3, got %s", result[0].Name())
	}
	if result[1].Name() != "c.mp3" {
		t.Errorf("expected c.mp3, got %s", result[1].Name())
	}
}

func TestFilterEntries_HidesHidden(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.mp3"), nil, 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	entries, _ := os.ReadDir(dir)
	filtered := filterEntries(entries, false)

	for _, e := range filtered {
		if e.Name() == ".hidden" {
			t.Error(".hidden should not appear when showHidden=false")
		}
	}
	names := make(map[string]bool)
	for _, e := range filtered {
		names[e.Name()] = true
	}
	if !names["visible.mp3"] {
		t.Error("visible.mp3 should be present")
	}
	if !names["subdir"] {
		t.Error("subdir should be present")
	}
}

func TestFilterEntries_ShowsHidden(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.mp3"), nil, 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	entries, _ := os.ReadDir(dir)
	filtered := filterEntries(entries, true)

	if len(filtered) != 3 {
		t.Errorf("expected 3 entries, got %d", len(filtered))
	}
}

func TestFilterEntries_AlwaysShowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.mp3")
	os.WriteFile(target, nil, 0644)
	link := filepath.Join(dir, "link.mp3")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	entries, _ := os.ReadDir(dir)
	filtered := filterEntries(entries, false)

	names := make(map[string]bool)
	for _, e := range filtered {
		names[e.Name()] = true
	}
	if !names["link.mp3"] {
		t.Error("symlink should be visible when showHidden=false")
	}
}

func TestToggleHidden_ReloadsDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "song.mp3"), nil, 0644)
	os.WriteFile(filepath.Join(dir, ".dotfile"), nil, 0644)

	m := newBrowserModel(dir)
	for _, e := range m.entries {
		if e.Name() == ".dotfile" {
			t.Error(".dotfile should be hidden initially")
		}
	}

	keyI := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
	m2, _ := m.Update(keyI)
	found := false
	for _, e := range m2.entries {
		if e.Name() == ".dotfile" {
			found = true
		}
	}
	if !found {
		t.Error(".dotfile should be visible after toggling i")
	}

	m3, _ := m2.Update(keyI)
	for _, e := range m3.entries {
		if e.Name() == ".dotfile" {
			t.Error(".dotfile should be hidden again after second toggle")
		}
	}
}

func TestFollowSymlinkToDir(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "realdir")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(target, "song.mp3"), nil, 0644)

	link := filepath.Join(root, "linkdir")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	m := newBrowserModel(root)

	// find the symlink entry index
	idx := -1
	for i, e := range m.entries {
		if e.Name() == "linkdir" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("linkdir not found in entries")
	}
	m.cursor = idx

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m2.dir != link {
		t.Errorf("expected dir %s, got %s", link, m2.dir)
	}
	if len(m2.entries) == 0 || m2.entries[0].Name() != "song.mp3" {
		t.Error("expected song.mp3 inside followed symlink dir")
	}
}

func TestDispatchCommand_EditSynonymForTag(t *testing.T) {
	dir := t.TempDir()
	makeMP3(t, dir, "song.mp3")

	m := newModel(dir, false)
	m.width = 80
	m.height = 24
	m.browser.height = 20

	_, teaCmd := dispatchCommand(m, "edit", nil)
	if teaCmd == nil {
		t.Fatal("expected a tea.Cmd from edit command")
	}
	msg := teaCmd()
	tagMsg, ok := msg.(execTagMsg)
	if !ok {
		t.Fatalf("expected execTagMsg, got %T", msg)
	}
	if len(tagMsg.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(tagMsg.files))
	}
}

func TestBrowseModeKey_E_OpensEditor(t *testing.T) {
	dir := t.TempDir()
	makeMP3(t, dir, "song.mp3")

	m := newModel(dir, false)
	m.width = 80
	m.height = 24
	m.browser.height = 20

	keyE := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")}
	result, teaCmd := m.Update(keyE)
	_ = result
	if teaCmd == nil {
		t.Fatal("expected a tea.Cmd from pressing e")
	}
	msg := teaCmd()
	if _, ok := msg.(execTagMsg); !ok {
		t.Errorf("expected execTagMsg, got %T", msg)
	}
}

func TestBrowseModeKey_C_NoFfmpeg(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "song.opus"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	m := newModel(dir, false) // ffmpeg unavailable
	m.width = 80
	m.height = 24
	m.browser.height = 20

	keyC := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	result, _ := m.Update(keyC)
	rm, ok := result.(model)
	if !ok {
		t.Fatal("expected model")
	}
	if !rm.statusIsError {
		t.Error("expected error status when ffmpeg unavailable")
	}
}

func TestParentNavRestoresCursor(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.mp3", "b.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Start in parent, navigate to subdir (it sorts first as a directory),
	// then press "h" to go back up.
	m := newBrowserModel(dir)
	m.height = 10

	// subdir should be at cursor 0 (dirs sort before files)
	if m.entries[0].Name() != "subdir" {
		t.Fatalf("expected subdir at index 0, got %s", m.entries[0].Name())
	}

	// Enter subdir
	m, _ = m.changeDir(sub)

	// Go back to parent via "h"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	if m.dir != dir {
		t.Fatalf("expected to be back in %s, got %s", dir, m.dir)
	}
	if m.entries[m.cursor].Name() != "subdir" {
		t.Errorf("expected cursor on subdir, got %s", m.entries[m.cursor].Name())
	}
}

func TestSelectAll_SelectsAllEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp3", "b.mp3", "c.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	m := newBrowserModel(dir)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})

	result := m.selectedEntries()
	if len(result) != 3 {
		t.Fatalf("expected 3 entries selected, got %d", len(result))
	}
	if result[0].Name() != "a.mp3" || result[1].Name() != "b.mp3" || result[2].Name() != "c.mp3" {
		t.Errorf("unexpected entry names: %v", result)
	}
}

func TestScrolling(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("file%02d.mp3", i)
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	m := newBrowserModel(dir)
	m.height = 3

	for i := 0; i < 5; i++ {
		m = m.scroll(1)
	}

	if m.cursor != 5 {
		t.Errorf("expected cursor 5, got %d", m.cursor)
	}
	if m.cursor >= m.offset+m.height {
		t.Errorf("cursor %d not visible in viewport [%d, %d)", m.cursor, m.offset, m.offset+m.height)
	}
}
