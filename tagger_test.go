package main

import (
	"os"
	"path/filepath"
	"testing"

	id3 "github.com/bogem/id3v2/v2"
)

// makeMP3 creates a file with a valid empty ID3 tag at the given path.
func makeMP3(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	tag, err := id3.Open(path, id3.Options{Parse: false})
	if err != nil {
		t.Fatalf("makeMP3 open: %v", err)
	}
	if err := tag.Save(); err != nil {
		tag.Close()
		t.Fatalf("makeMP3 save: %v", err)
	}
	tag.Close()
	return path
}

func TestTagReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := makeMP3(t, dir, "test.mp3")

	m, err := newTaggerModel([]string{path})
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}

	m.fields[0].value = "Test Title"
	m.fields[1].value = "Test Artist"

	msg := m.saveTags()()
	if _, ok := msg.(tagSavedMsg); !ok {
		t.Fatalf("expected tagSavedMsg, got %T: %v", msg, msg)
	}

	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer tag.Close()

	if got := tag.Title(); got != "Test Title" {
		t.Errorf("title: got %q, want %q", got, "Test Title")
	}
	if got := tag.Artist(); got != "Test Artist" {
		t.Errorf("artist: got %q, want %q", got, "Test Artist")
	}
}

func TestBulkTag_BlankFieldSkipped(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		makeMP3(t, dir, "a.mp3"),
		makeMP3(t, dir, "b.mp3"),
	}

	// Pre-populate Artist on both files.
	for _, p := range paths {
		tag, err := id3.Open(p, id3.Options{Parse: true})
		if err != nil {
			t.Fatalf("setup open: %v", err)
		}
		tag.SetArtist("Original Artist")
		if err := tag.Save(); err != nil {
			tag.Close()
			t.Fatalf("setup save: %v", err)
		}
		tag.Close()
	}

	// Bulk-tag: fill Title only, leave Artist blank.
	m, err := newTaggerModel(paths)
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}
	m.fields[0].value = "Bulk Title" // Title

	msg := m.saveTags()()
	bm, ok := msg.(tagBulkSavedMsg)
	if !ok {
		t.Fatalf("expected tagBulkSavedMsg, got %T", msg)
	}
	if bm.count != 2 {
		t.Errorf("expected count 2, got %d", bm.count)
	}

	// Artist must be unchanged on both files.
	for _, p := range paths {
		tag, err := id3.Open(p, id3.Options{Parse: true})
		if err != nil {
			t.Fatalf("verify open: %v", err)
		}
		artist := tag.Artist()
		tag.Close()
		if artist != "Original Artist" {
			t.Errorf("%s: artist = %q, want %q", filepath.Base(p), artist, "Original Artist")
		}
	}
}
