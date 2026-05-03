package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
		m = m.scrollDown()
	}

	if m.cursor != 5 {
		t.Errorf("expected cursor 5, got %d", m.cursor)
	}
	if m.cursor >= m.offset+m.height {
		t.Errorf("cursor %d not visible in viewport [%d, %d)", m.cursor, m.offset, m.offset+m.height)
	}
}
