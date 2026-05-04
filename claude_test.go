package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
