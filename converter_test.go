package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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

	msg := convertFile(context.Background(), src, dir)()
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

func TestConvertOggToMp3(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyOggFixture(t, dir)

	msg := convertFile(context.Background(), src, dir)()
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

func TestConvertSkipsExisting(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	src := copyFixture(t, dir)

	if _, ok := convertFile(context.Background(), src, dir)().(convertDoneMsg); !ok {
		t.Fatal("first convert: expected convertDoneMsg")
	}

	msg := convertFile(context.Background(), src, dir)()
	if _, ok := msg.(convertSkippedMsg); !ok {
		t.Fatalf("second convert: expected convertSkippedMsg, got %T", msg)
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
