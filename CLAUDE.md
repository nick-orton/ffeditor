# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Commands

```bash
make            # build + test (default target)
make build      # compile to ./ffeditor
make test       # go test ./... (includes ffmpeg integration tests)
make test-short # go test -short ./... (unit tests only, skips ffmpeg)
```

Run a single test:
```bash
go test -run TestTagReadWrite .
go test -short -run TestClaudeGuessTagsCmd .
```

## Documentation

Documentation on the design and architecture of the app is found in docs/

```
docs/
  ├── architecture.md   (Describes the system design and patterns)
  ├── requirements.md   (initial requirements, since superceded by github 
  |                      issues for new features)
  └── design.md         (Describes how the system should behave)
README.md               (user-facing documentation)
```

Documentation should be updated with new changes

## Architecture

All source is in `package main` (flat structure, no sub-packages). The app is 
a Bubble Tea TUI.

**Mode state machine** (in `model.go`):
- `modeBrowse` → `modeCommand` (`:`) → dispatches `execConvertMsg` / 
  `execTagMsg` / `dirChangedMsg`
- `modeBrowse` → `modeHelp` (`?`)
- `modeTag` → `modeTagSaving` (Ctrl+S) / `modeTagSearching` (Ctrl+T)
- All modes collapse back to `modeBrowse` on completion/cancel

**Concurrency rules:**
- File I/O (conversion, tag writes, API calls) always runs in a `tea.Cmd` 
  goroutine, never directly in `Update`.
- Directory reads (`os.ReadDir`) run synchronously in `Update`.
- Conversions are sequential (one ffmpeg process at a time); cancellation uses 
  `context.WithCancel`.

**Key design decisions:**
- `selectedEntries()` in `browser.go` returns cursor entry when nothing is 
  selected — all commands use this for unified single/multi handling.
- Bulk tag mode disables the Title field (index 0) to prevent accidental 
  overwrites; `focusIndex` starts at 1 (Artist) and navigation wraps 1–5.
- `claudeAPIURL` in `claude.go` is a package-level var overridden in tests to 
  avoid real API calls.
- Enter key intentionally does NOT save tags — Ctrl+S only.

## Runtime Requirements

- `ffmpeg` on `$PATH` — optional; conversion is disabled if absent
- `ANTHROPIC_API_KEY` env var — optional; Ctrl+T smart tag lookup shows error if unset
- No CGo (pure-Go ID3 library: `github.com/bogem/id3v2/v2`)
