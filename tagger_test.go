package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	id3 "github.com/bogem/id3v2/v2"
	flac "github.com/go-flac/go-flac"
	flacvorbis "github.com/go-flac/flacvorbis"
	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

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

func writeTag(t *testing.T, path, title, artist, album, year string) {
	t.Helper()
	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		t.Fatalf("writeTag open: %v", err)
	}
	tag.SetTitle(title)
	tag.SetArtist(artist)
	tag.SetAlbum(album)
	tag.SetYear(year)
	if err := tag.Save(); err != nil {
		tag.Close()
		t.Fatalf("writeTag save: %v", err)
	}
	tag.Close()
}

func TestBulkSharedTags_Prefilled(t *testing.T) {
	dir := t.TempDir()
	a := makeMP3(t, dir, "a.mp3")
	b := makeMP3(t, dir, "b.mp3")
	writeTag(t, a, "Song A", "Same Artist", "Same Album", "2001")
	writeTag(t, b, "Song B", "Same Artist", "Same Album", "2001")

	m, err := newTaggerModel([]string{a, b})
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}

	// Artist, Album, Year shared → prefilled.
	if m.fields[1].value != "Same Artist" {
		t.Errorf("artist: got %q, want %q", m.fields[1].value, "Same Artist")
	}
	if m.fields[2].value != "Same Album" {
		t.Errorf("album: got %q, want %q", m.fields[2].value, "Same Album")
	}
	if m.fields[3].value != "2001" {
		t.Errorf("year: got %q, want %q", m.fields[3].value, "2001")
	}

	// Title differs → blank.
	if m.fields[0].value != "" {
		t.Errorf("title should be blank when files differ, got %q", m.fields[0].value)
	}
}

func TestBulkSharedTags_Disagreement(t *testing.T) {
	dir := t.TempDir()
	a := makeMP3(t, dir, "a.mp3")
	b := makeMP3(t, dir, "b.mp3")
	writeTag(t, a, "", "Artist A", "", "")
	writeTag(t, b, "", "Artist B", "", "")

	m, err := newTaggerModel([]string{a, b})
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}
	if m.fields[1].value != "" {
		t.Errorf("artist should be blank when files disagree, got %q", m.fields[1].value)
	}
}

func TestBulkMode_FocusStartsAtArtist(t *testing.T) {
	dir := t.TempDir()
	paths := []string{makeMP3(t, dir, "a.mp3"), makeMP3(t, dir, "b.mp3")}
	m, err := newTaggerModel(paths)
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}
	if m.focusIndex != 1 {
		t.Errorf("focusIndex: got %d, want 1 (Artist)", m.focusIndex)
	}
}

func TestBulkMode_NavigationSkipsTitle(t *testing.T) {
	dir := t.TempDir()
	paths := []string{makeMP3(t, dir, "a.mp3"), makeMP3(t, dir, "b.mp3")}
	m, err := newTaggerModel(paths)
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}

	// Pressing up from Artist (1) should wrap to Genre (5), skipping Title (0).
	m, _ = m.Update(keyMsg("up"))
	if m.focusIndex != 5 {
		t.Errorf("up from Artist: got focusIndex %d, want 5 (Genre)", m.focusIndex)
	}

	// Pressing down from Genre (5) should go to Artist (1), skipping Title (0).
	m, _ = m.Update(keyMsg("down"))
	if m.focusIndex != 1 {
		t.Errorf("down from Genre: got focusIndex %d, want 1 (Artist)", m.focusIndex)
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

// makeFlac creates a minimal valid FLAC file at the given path.
// It writes an empty StreamInfo block followed by fake frame sync bytes so
// that go-flac's ParseFile (which reads the audio stream) accepts the file.
func makeFlac(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)

	// Build a minimal FLAC binary from scratch.
	var buf bytes.Buffer

	// Magic header.
	buf.WriteString("fLaC")

	// StreamInfo metadata block: last-block=1, type=0 (StreamInfo), length=34.
	// Header: bit7=last, bits[6:1]=type(0), bits[23:0]=length(34=0x22).
	buf.Write([]byte{0x80, 0x00, 0x00, 0x22})
	// StreamInfo data: 34 zero bytes (sample rate=0 is technically invalid but
	// go-flac does not validate these values when parsing).
	buf.Write(make([]byte, 34))

	// Minimal audio frame starting with FLAC sync code 0xFF 0xF8.
	buf.Write([]byte{0xFF, 0xF8, 0x18, 0x00})

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeFlacTags writes the given key=value pairs into the Vorbis Comment block
// of a FLAC file, used to set up test fixtures.
func writeFlacTags(t *testing.T, path string, tags map[string]string) {
	t.Helper()
	f, err := flac.ParseFile(path)
	if err != nil {
		t.Fatalf("writeFlacTags ParseFile: %v", err)
	}

	// Find or create the Vorbis Comment block.
	var cmt *flacvorbis.MetaDataBlockVorbisComment
	idx := -1
	for i, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			idx = i
			break
		}
	}
	if idx >= 0 {
		cmt, err = flacvorbis.ParseFromMetaDataBlock(*f.Meta[idx])
		if err != nil {
			cmt = flacvorbis.New()
		}
	} else {
		cmt = flacvorbis.New()
	}

	for key, val := range tags {
		if err := cmt.Add(key, val); err != nil {
			t.Fatalf("writeFlacTags Add: %v", err)
		}
	}

	block := cmt.Marshal()
	if idx >= 0 {
		f.Meta[idx] = &block
	} else {
		f.Meta = append(f.Meta, &block)
	}
	if err := f.Save(path); err != nil {
		t.Fatalf("writeFlacTags Save: %v", err)
	}
}

func TestFLACTagReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := makeFlac(t, dir, "test.flac")

	m, err := newTaggerModel([]string{path})
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}

	m.fields[0].value = "FLAC Title"
	m.fields[1].value = "FLAC Artist"

	msg := m.saveTags()()
	if _, ok := msg.(tagSavedMsg); !ok {
		t.Fatalf("expected tagSavedMsg, got %T: %v", msg, msg)
	}

	data, err := readFLACTags(path)
	if err != nil {
		t.Fatalf("readFLACTags: %v", err)
	}
	if data.Title != "FLAC Title" {
		t.Errorf("title: got %q, want %q", data.Title, "FLAC Title")
	}
	if data.Artist != "FLAC Artist" {
		t.Errorf("artist: got %q, want %q", data.Artist, "FLAC Artist")
	}
}

func TestFLACTagPreload(t *testing.T) {
	dir := t.TempDir()
	path := makeFlac(t, dir, "tagged.flac")
	writeFlacTags(t, path, map[string]string{
		flacvorbis.FIELD_TITLE:  "Existing Title",
		flacvorbis.FIELD_ARTIST: "Existing Artist",
	})

	m, err := newTaggerModel([]string{path})
	if err != nil {
		t.Fatalf("newTaggerModel: %v", err)
	}
	if m.fields[0].value != "Existing Title" {
		t.Errorf("title: got %q, want %q", m.fields[0].value, "Existing Title")
	}
	if m.fields[1].value != "Existing Artist" {
		t.Errorf("artist: got %q, want %q", m.fields[1].value, "Existing Artist")
	}
}

func TestFLACTagSummaryInBrowser(t *testing.T) {
	dir := t.TempDir()
	path := makeFlac(t, dir, "song.flac")
	writeFlacTags(t, path, map[string]string{
		flacvorbis.FIELD_ARTIST: "Test Artist",
		flacvorbis.FIELD_TITLE:  "Test Song",
	})

	summary := readTagSummary(path)
	if summary.artist != "Test Artist" {
		t.Errorf("artist: got %q, want %q", summary.artist, "Test Artist")
	}
	if summary.title != "Test Song" {
		t.Errorf("title: got %q, want %q", summary.title, "Test Song")
	}
}
