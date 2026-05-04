package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tagSearchResultMsg struct {
	artist string
	title  string
	year   string
}

type tagSearchErrMsg struct{ err error }

func claudeGuessTagsCmd(filename string) tea.Cmd {
	return func() tea.Msg {
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return tagSearchErrMsg{errors.New("ANTHROPIC_API_KEY not set")}
		}

		reqBody, err := json.Marshal(map[string]any{
			"model":      "claude-haiku-4-5-20251001",
			"max_tokens": 100,
			"system":     `Extract music metadata from a filename. Reply ONLY with a JSON object with keys "artist", "title", "year". Use empty string if unknown.`,
			"messages": []map[string]string{
				{"role": "user", "content": filepath.Base(filename)},
			},
		})
		if err != nil {
			return tagSearchErrMsg{err}
		}

		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
		if err != nil {
			return tagSearchErrMsg{err}
		}
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return tagSearchErrMsg{err}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return tagSearchErrMsg{err}
		}
		if resp.StatusCode != http.StatusOK {
			return tagSearchErrMsg{fmt.Errorf("API error %d: %s", resp.StatusCode, body)}
		}

		// Parse Messages API envelope.
		var envelope struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return tagSearchErrMsg{err}
		}
		if len(envelope.Content) == 0 {
			return tagSearchErrMsg{errors.New("empty response from API")}
		}

		text := envelope.Content[0].Text
		// Extract the JSON object robustly.
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start < 0 || end < 0 || end < start {
			return tagSearchErrMsg{fmt.Errorf("no JSON in response: %s", text)}
		}
		jsonStr := text[start : end+1]

		var result struct {
			Artist string `json:"artist"`
			Title  string `json:"title"`
			Year   string `json:"year"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return tagSearchErrMsg{err}
		}

		return tagSearchResultMsg{
			artist: result.Artist,
			title:  result.Title,
			year:   result.Year,
		}
	}
}
