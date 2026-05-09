package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	id3 "github.com/bogem/id3v2/v2"
)

func TestClaudeGuessTagsCmd_NoAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	msg := claudeGuessTagsCmd("some/file.mp3")()
	if _, ok := msg.(tagSearchErrMsg); !ok {
		t.Fatalf("expected tagSearchErrMsg, got %T", msg)
	}
}

func TestClaudeGuessTagsCmd_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"{\"artist\":\"The Beatles\",\"title\":\"Hey Jude\",\"year\":\"1968\"}"}]
		}`))
	}))
	defer srv.Close()

	orig := claudeAPIURL
	claudeAPIURL = srv.URL
	defer func() { claudeAPIURL = orig }()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	msg := claudeGuessTagsCmd("Hey Jude - The Beatles - 1968.mp3")()
	result, ok := msg.(tagSearchResultMsg)
	if !ok {
		t.Fatalf("expected tagSearchResultMsg, got %T", msg)
	}
	if result.artist != "The Beatles" {
		t.Errorf("artist: got %q, want %q", result.artist, "The Beatles")
	}
	if result.title != "Hey Jude" {
		t.Errorf("title: got %q, want %q", result.title, "Hey Jude")
	}
	if result.year != "1968" {
		t.Errorf("year: got %q, want %q", result.year, "1968")
	}
}

func TestClaudeGuessTagsCmd_JSONEmbeddedInProse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"Here you go: {\"artist\":\"Radiohead\",\"title\":\"Creep\",\"year\":\"1992\"} Hope that helps!"}]
		}`))
	}))
	defer srv.Close()

	orig := claudeAPIURL
	claudeAPIURL = srv.URL
	defer func() { claudeAPIURL = orig }()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	msg := claudeGuessTagsCmd("creep.mp3")()
	result, ok := msg.(tagSearchResultMsg)
	if !ok {
		t.Fatalf("expected tagSearchResultMsg, got %T: %v", msg, msg)
	}
	if result.artist != "Radiohead" {
		t.Errorf("artist: got %q, want %q", result.artist, "Radiohead")
	}
}

func TestClaudeGuessTagsCmd_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	orig := claudeAPIURL
	claudeAPIURL = srv.URL
	defer func() { claudeAPIURL = orig }()

	t.Setenv("ANTHROPIC_API_KEY", "bad-key")

	msg := claudeGuessTagsCmd("file.mp3")()
	if _, ok := msg.(tagSearchErrMsg); !ok {
		t.Fatalf("expected tagSearchErrMsg, got %T", msg)
	}
}

func TestSmartTagCmd_NoAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	dir := t.TempDir()
	path := makeMP3(t, dir, "test.mp3")
	msg := smartTagCmd([]string{path})()
	if _, ok := msg.(smartTagErrMsg); !ok {
		t.Fatalf("expected smartTagErrMsg, got %T", msg)
	}
}

func TestSmartTagCmd_FillsMissingFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"{\"artist\":\"The Beatles\",\"title\":\"Hey Jude\",\"year\":\"1968\"}"}]
		}`))
	}))
	defer srv.Close()

	orig := claudeAPIURL
	claudeAPIURL = srv.URL
	defer func() { claudeAPIURL = orig }()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	dir := t.TempDir()
	path := makeMP3(t, dir, "hey-jude-beatles-1968.mp3")

	msg := smartTagCmd([]string{path})()
	result, ok := msg.(smartTagDoneMsg)
	if !ok {
		t.Fatalf("expected smartTagDoneMsg, got %T", msg)
	}
	if result.count != 1 {
		t.Errorf("count: got %d, want 1", result.count)
	}

	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer tag.Close()
	if got := tag.Title(); got != "Hey Jude" {
		t.Errorf("title: got %q, want %q", got, "Hey Jude")
	}
	if got := tag.Artist(); got != "The Beatles" {
		t.Errorf("artist: got %q, want %q", got, "The Beatles")
	}
	if got := tag.Year(); got != "1968" {
		t.Errorf("year: got %q, want %q", got, "1968")
	}
}

func TestSmartTagCmd_DoesNotOverwriteExistingFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"{\"artist\":\"Wrong Artist\",\"title\":\"Wrong Title\",\"year\":\"2000\"}"}]
		}`))
	}))
	defer srv.Close()

	orig := claudeAPIURL
	claudeAPIURL = srv.URL
	defer func() { claudeAPIURL = orig }()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	dir := t.TempDir()
	path := makeMP3(t, dir, "test.mp3")

	// Pre-populate artist only; title and year are missing.
	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	tag.SetArtist("Correct Artist")
	if err := tag.Save(); err != nil {
		tag.Close()
		t.Fatalf("setup save: %v", err)
	}
	tag.Close()

	msg := smartTagCmd([]string{path})()
	if _, ok := msg.(smartTagDoneMsg); !ok {
		t.Fatalf("expected smartTagDoneMsg, got %T", msg)
	}

	tag, err = id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer tag.Close()

	// Artist must be unchanged.
	if got := tag.Artist(); got != "Correct Artist" {
		t.Errorf("artist: got %q, want %q (must not be overwritten)", got, "Correct Artist")
	}
	// Title and year should be filled by the API.
	if got := tag.Title(); got != "Wrong Title" {
		t.Errorf("title: got %q, want %q", got, "Wrong Title")
	}
	if got := tag.Year(); got != "2000" {
		t.Errorf("year: got %q, want %q", got, "2000")
	}
}

func TestSmartTagCmd_SkipsFullyTaggedFiles(t *testing.T) {
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": [{"type":"text","text":"{}"}]}`))
	}))
	defer srv.Close()

	orig := claudeAPIURL
	claudeAPIURL = srv.URL
	defer func() { claudeAPIURL = orig }()

	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	dir := t.TempDir()
	path := makeMP3(t, dir, "test.mp3")
	writeTag(t, path, "Title", "Artist", "", "2001")

	msg := smartTagCmd([]string{path})()
	result, ok := msg.(smartTagDoneMsg)
	if !ok {
		t.Fatalf("expected smartTagDoneMsg, got %T", msg)
	}
	if result.count != 0 {
		t.Errorf("count: got %d, want 0 (file already fully tagged)", result.count)
	}
	if apiCalled {
		t.Error("API should not be called for fully-tagged files")
	}
}
