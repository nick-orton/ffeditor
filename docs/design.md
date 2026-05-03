# FFeditor

[Requirements](./requirements.md)

```
project-name: ffeditor
UI:           TUI
Language:     go
```

## Overview

A terminal-based (TUI) tool for managing a personal digital music collection. 
The user navigates their filesystem, converts audio formats, and edits ID3 
metadata — all from within a single interface.

## Technology

- **Language:** Go
- **TUI framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) 
  (with Lip Gloss for styling)
- **Audio conversion:** shells out to `ffmpeg` (must be installed on the host)
- **ID3 tagging:** [go-taglib](https://github.com/nicfit/go-taglib) or 
  [id3v2](https://github.com/bogem/id3v2) pure-Go library

## Architecture

```
main.go              — entry point, arg parsing, Bubble Tea program init
model.go             — top-level TUI model, update loop, view
browser.go           — file-system browser component
converter.go         — audio conversion logic (ffmpeg wrapper)
tagger.go            — ID3 tag read/write logic
commands.go          — command-bar parsing & dispatch
```

## TUI Layout

```
┌─────────────────────────────────────────────┐
│  FFEditor                  /home/user/Music │  ← header / current path
├─────────────────────────────────────────────┤
│  ▸ Albums/                                  │
│    Tracks/                                  │
│    cover.jpg                                │
│    song.opus                                │
│    demo.m4a                                 │  ← file browser (scrollable)
│    notes.txt                                │
│                                             │
├─────────────────────────────────────────────┤
│  [status / progress messages]               │  ← status bar
├─────────────────────────────────────────────┤
│  > _                                        │  ← command input
└─────────────────────────────────────────────┘
```

### Navigation

| Key       | Action                              |
|-----------|-------------------------------------|
| `↑` / `k` | Move cursor up                     |
| `↓` / `j` | Move cursor down                   |
| `Enter`   | Open directory / select file        |
| `h` | Go to parent directory            |
| `:`       | Focus command bar                   |
| `q`       | Quit                                |
| `Space`   | Toggle selection (for bulk ops)     |

## Features

### 1. File Browser

- Lists files and directories in the current path.
- Directories sort first, then files alphabetically.
- Audio files (`.mp3`, `.opus`, `.m4a`, `.flac`, `.ogg`) are visually 
  highlighted.
- Multi-select with `Space` for bulk operations.

### 2. Audio Conversion

Converts `.opus` and `.m4a` files to `.mp3` by shelling out to `ffmpeg`.

#### Single file

Select a file and run `:convert`. The tool executes:

```
ffmpeg -i input.opus -codec:a libmp3lame -qscale:a 2 output.mp3
```

- Output file is placed in the same directory with the `.mp3` extension.
- Source file is kept (not deleted) by default.
- ID3 tags are preserved/copied when possible (ffmpeg handles this automatically for most metadata).

#### Bulk convert

Select a directory (or multi-select files) and run `:convert`.

- Recursively finds all `.opus` and `.m4a` files in the selection.
- Converts each file sequentially, showing a progress count in the status bar (e.g., `Converting 3/17...`).
- Skips files that already have a corresponding `.mp3` sibling.
- On error, logs the failure to the status bar and continues with the next file.

### 3. ID3 Tag Editing

Select an `.mp3` file and run `:tag` to enter tag-editing mode.

#### Tag editing view

```
┌─────────────────────────────────────────────┐
│  Editing tags: song.mp3                     │
├─────────────────────────────────────────────┤
│  Title:   My Song                           │
│  Artist:  Some Artist                       │
│  Album:   Great Album                       │
│  Year:    2024                              │
│  Track:   3                                 │
│  Genre:   Rock                              │
├─────────────────────────────────────────────┤
│  Tab: next field  Enter: save  Esc: cancel  │
└─────────────────────────────────────────────┘
```

- Fields are pre-populated with existing tag values.
- `Tab` / `Shift+Tab` cycles between fields.
- `Enter` writes all fields back to the file.
- `Esc` discards changes and returns to the browser.

#### Bulk tagging

Multi-select several `.mp3` files, then run `:tag`. Only fields the user fills 
in are written; blank fields are left unchanged on each file. Useful for 
setting a shared album or artist across multiple tracks.

## Commands

All commands are entered via the `:` command bar.

| Command          | Description                                       |
|------------------|---------------------------------------------------|
| `:convert`       | Convert selected file(s) from opus/m4a to mp3     |
| `:tag`           | Open ID3 tag editor for selected mp3 file(s)      |
| `:cd <path>`     | Change the browser to an absolute or relative path |
| `:q`             | Quit                                               |

## Error Handling

- If `ffmpeg` is not found on `$PATH`, display an error on startup and disable 
  conversion commands.
- Conversion or tagging errors are shown in the status bar; they do not crash 
  the application.
- Permission errors (read-only files/directories) are surfaced in the status 
  bar.

## Future Considerations (out of scope for v1)

- Playback preview (e.g., via `mpv` or `ffplay`).
- FLAC/OGG conversion support.
- Album art embedding.
- Batch rename files from tag data.
