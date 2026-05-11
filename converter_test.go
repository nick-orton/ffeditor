package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	id3 "github.com/bogem/id3v2/v2"
)

func TestMain(m *testing.M) {
	// Generate test fixtures once if ffmpeg is available.
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		_ = os.MkdirAll("testdata", 0755)

		const opusFixture = "testdata/silence.opus"
		if _, err := os.Stat(opusFixture); os.IsNotExist(err) {
			cmd := exec.Command("ffmpeg",
				"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono",
				"-t", "0.5", "-c:a", "libopus", opusFixture)
			cmd.Stdout = nil
			cmd.Stderr = nil
			_ = cmd.Run()
		}

		const oggFixture = "testdata/silence.ogg"
		if _, err := os.Stat(oggFixture); os.IsNotExist(err) {
			cmd := exec.Command("ffmpeg",
				"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono",
				"-t", "0.5", "-c:a", "libvorbis", oggFixture)
			cmd.Stdout = nil
			cmd.Stderr = nil
			_ = cmd.Run()
		}

		const aacFixture = "testdata/silence.aac"
		if _, err := os.Stat(aacFixture); os.IsNotExist(err) {
			cmd := exec.Command("ffmpeg",
				"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono",
				"-t", "0.5", "-c:a", "aac", aacFixture)
			cmd.Stdout = nil
			cmd.Stderr = nil
			_ = cmd.Run()
		}

		const wavFixture = "testdata/silence.wav"
		if _, err := os.Stat(wavFixture); os.IsNotExist(err) {
			cmd := exec.Command("ffmpeg",
				"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono",
				"-t", "0.5", wavFixture)
			cmd.Stdout = nil
			cmd.Stderr = nil
			_ = cmd.Run()
		}
	}
	os.Exit(m.Run())
}

// skipIfNoFFmpeg skips the test when ffmpeg is absent or -short is set.
func skipIfNoFFmpeg(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found")
	}
	if _, err := os.Stat("testdata/silence.opus"); os.IsNotExist(err) {
		t.Skip("testdata/silence.opus not found")
	}
}

// copyOggFixture copies testdata/silence.ogg into dst and returns the new path.
func copyOggFixture(t *testing.T, dst string) string {
	t.Helper()
	if _, err := os.Stat("testdata/silence.ogg"); os.IsNotExist(err) {
		t.Skip("testdata/silence.ogg not found")
	}
	in, err := os.Open("testdata/silence.ogg")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	dest := filepath.Join(dst, "silence.ogg")
	out, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	return dest
}

// copyFixture copies testdata/silence.opus into dst and returns the new path.
func copyFixture(t *testing.T, dst string) string {
	t.Helper()
	in, err := os.Open("testdata/silence.opus")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	dest := filepath.Join(dst, "silence.opus")
	out, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestConvertOpusToMp3(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyFixture(t, dir)

	msg := convertFile(context.Background(), src)()
	done, ok := msg.(convertDoneMsg)
	if !ok {
		t.Fatalf("expected convertDoneMsg, got %T: %v", msg, msg)
	}

	info, err := os.Stat(done.dest)
	if err != nil {
		t.Fatalf("mp3 not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output mp3 is empty")
	}
}

// copyAacFixture copies testdata/silence.aac into dst and returns the new path.
func copyAacFixture(t *testing.T, dst string) string {
	t.Helper()
	if _, err := os.Stat("testdata/silence.aac"); os.IsNotExist(err) {
		t.Skip("testdata/silence.aac not found")
	}
	in, err := os.Open("testdata/silence.aac")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	dest := filepath.Join(dst, "silence.aac")
	out, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestConvertOggToMp3(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyOggFixture(t, dir)

	msg := convertFile(context.Background(), src)()
	done, ok := msg.(convertDoneMsg)
	if !ok {
		t.Fatalf("expected convertDoneMsg, got %T: %v", msg, msg)
	}

	info, err := os.Stat(done.dest)
	if err != nil {
		t.Fatalf("mp3 not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output mp3 is empty")
	}
}

func TestConvertAacToMp3(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyAacFixture(t, dir)

	msg := convertFile(context.Background(), src)()
	done, ok := msg.(convertDoneMsg)
	if !ok {
		t.Fatalf("expected convertDoneMsg, got %T: %v", msg, msg)
	}

	info, err := os.Stat(done.dest)
	if err != nil {
		t.Fatalf("mp3 not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output mp3 is empty")
	}
}

// copyWavFixture copies testdata/silence.wav into dst and returns the new path.
func copyWavFixture(t *testing.T, dst string) string {
	t.Helper()
	if _, err := os.Stat("testdata/silence.wav"); os.IsNotExist(err) {
		t.Skip("testdata/silence.wav not found")
	}
	in, err := os.Open("testdata/silence.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	dest := filepath.Join(dst, "silence.wav")
	out, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestConvertWavToFlac(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyWavFixture(t, dir)

	msg := convertFile(context.Background(), src)()
	done, ok := msg.(convertDoneMsg)
	if !ok {
		t.Fatalf("expected convertDoneMsg, got %T: %v", msg, msg)
	}

	if filepath.Ext(done.dest) != ".flac" {
		t.Errorf("expected .flac output, got %q", done.dest)
	}

	info, err := os.Stat(done.dest)
	if err != nil {
		t.Fatalf("flac not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output flac is empty")
	}
}


func TestConvertSkipsExisting(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyFixture(t, dir)

	if _, ok := convertFile(context.Background(), src)().(convertDoneMsg); !ok {
		t.Fatal("first convert: expected convertDoneMsg")
	}

	msg := convertFile(context.Background(), src)()
	if _, ok := msg.(convertSkippedMsg); !ok {
		t.Fatalf("second convert: expected convertSkippedMsg, got %T", msg)
	}
}

func TestConvertCopiesMetadata(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "tagged.opus")

	// Create an opus file with embedded metadata.
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono",
		"-t", "0.5", "-c:a", "libopus",
		"-metadata", "title=TestTitle",
		"-metadata", "artist=TestArtist",
		"-metadata", "album=TestAlbum",
		src)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create tagged opus: %v", err)
	}

	msg := convertFile(context.Background(), src)()
	done, ok := msg.(convertDoneMsg)
	if !ok {
		t.Fatalf("expected convertDoneMsg, got %T: %v", msg, msg)
	}

	tag, err := id3.Open(done.dest, id3.Options{Parse: true})
	if err != nil {
		t.Fatalf("failed to open output mp3: %v", err)
	}
	defer tag.Close()

	if tag.Title() != "TestTitle" {
		t.Errorf("title: got %q, want %q", tag.Title(), "TestTitle")
	}
	if tag.Artist() != "TestArtist" {
		t.Errorf("artist: got %q, want %q", tag.Artist(), "TestArtist")
	}
	if tag.Album() != "TestAlbum" {
		t.Errorf("album: got %q, want %q", tag.Album(), "TestAlbum")
	}
}

func TestBuildConvertList_Dedup(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "music")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "song.opus"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Duplicate the directory entry to exercise the seen-path dedup.
	entries = append(entries, entries...)

	result := buildConvertList(entries, dir)

	seen := make(map[string]bool)
	for _, p := range result {
		if seen[p] {
			t.Errorf("duplicate path in result: %s", p)
		}
		seen[p] = true
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d: %v", len(result), result)
	}
}
