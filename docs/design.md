# FFeditor

[Requirements](./requirements.md)

```text
project-name: ffeditor
UI:           TUI
Language:     go
```

## Overview

A terminal-based (TUI) tool for managing a personal digital music
collection. The user navigates their filesystem, converts audio
formats, and edits ID3 metadata — all from within a single interface.

## Technology

- **Language:** Go
- **TUI framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea)
  (with Lip Gloss for styling)
- **Audio conversion:** shells out to `ffmpeg` (must be installed on
  the host)
- **ID3 tagging:** [bogem/id3v2](https://github.com/bogem/id3v2)
  pure-Go library

## Architecture

```text
main.go              — entry point, arg parsing, Bubble Tea program init
model.go             — top-level TUI model, update loop, view
browser.go           — file-system browser component
converter.go         — audio conversion logic (ffmpeg wrapper)
tagger.go            — ID3 tag read/write logic
commands.go          — command-bar parsing & dispatch
```

## TUI Layout

```text
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

| Key           | Action                                          |
|---------------|-------------------------------------------------|
| `↑` / `k`     | Move cursor up                                  |
| `↓` / `j`     | Move cursor down                                |
| `Enter` / `l` | Enter directory or follow symlink to directory  |
| `h`           | Go to parent directory                          |
| `i`           | Toggle hidden files (dotfiles)                  |
| `Space`       | Toggle selection (for bulk ops), advance cursor |
| `:`           | Focus command bar                               |
| `Ctrl+C`      | Cancel in-progress conversion (stay in app)     |
| `q`           | Quit                                            |

## Features

### 1. File Browser

- Lists files and directories in the current path.
- Directories sort first, then files alphabetically.
- Audio files (`.mp3`, `.opus`, `.m4a`, `.flac`, `.ogg`) are visually
  highlighted.
- Dotfiles (dot-prefixed names) are hidden by default. Press `i` to
  toggle them on; when visible they render dimmed.
- Symlinks are shown with a trailing `@` in cyan. Symlinks that point
  to a directory show `@/` and can be entered with `l` or `Enter`.
- Multi-select with `Space` for bulk operations; selection clears on
  directory change.
- When nothing is explicitly selected, commands operate on the entry
  under the cursor.

### 2. Audio Conversion

Converts `.opus` and `.m4a` files to `.mp3` by shelling out to
`ffmpeg`.

#### Single file

Place the cursor on a file (or `Space`-select it) and run `:convert`.
The tool executes:

```text
ffmpeg -y -i input.opus -codec:a libmp3lame -qscale:a 2 output.mp3
```

- Output file is placed in the same directory with the `.mp3`
  extension.
- Source file is kept (not deleted).
- ID3 tags are carried over automatically by ffmpeg where supported.

#### Bulk convert

Select a directory (or multi-select files) and run `:convert`.

- Recursively finds all `.opus` and `.m4a` files in the selection.
- Duplicate paths are deduplicated before conversion begins.
- Converts each file sequentially (one ffmpeg process at a time),
  showing a progress count in the status bar: `Converting 3/17...`
- Skips files that already have a corresponding `.mp3` in the same
  directory.
- On error, records the failure and continues with the next file.
- On completion:
  `Conversion complete (N converted, M skipped, E errors)`.
- The browser directory is refreshed automatically when conversion
  finishes so new `.mp3` files appear immediately.

#### Cancellation

Press `Ctrl+C` during a conversion to kill the current ffmpeg process.
The application stays open, the status bar shows
`Conversion cancelled`, and the browser refreshes. Files already
converted before cancellation are kept.

### 3. ID3 Tag Editing

Select an `.mp3` file and run `:tag` to enter tag-editing mode.

#### Tag editing view

```text
╭─ Files ─────────────────────────────────────╮
│ song.mp3                                    │
╰─────────────────────────────────────────────╯

╭─ Tags ──────────────────────────────────────╮
│     Title: My Song▌                         │
│    Artist: Some Artist                      │
│     Album: Great Album                      │
│      Year: 2024                             │
│     Track: 3                                │
│     Genre: Rock                             │
╰─────────────────────────────────────────────╯

  Up/Down: navigate   Tab: complete   Ctrl+S: save   Esc: cancel
```

- The Files box lists the file(s) being edited; the Tags box shows the
  six editable fields. Both boxes are drawn with rounded borders in
  the header color.
- Fields are pre-populated with existing tag values for a single file.
- `↑` / `↓` moves between fields (wraps around). `Shift+Tab` also
  moves up.
- Typing appends to the focused field; `Backspace` deletes the last
  character.
- `Tab` completes the current word being typed using tokens extracted
  from the filename(s). The filename is split on non-alphanumeric
  characters (spaces, underscores, hyphens, etc.) to produce the token
  list. Repeated `Tab` presses cycle through all matching tokens; any
  edit resets the cycle.
- `Ctrl+S` writes changed fields back to the file and returns to the
  browser.
- `Esc` discards changes and returns to the browser.

#### Bulk tagging

Multi-select several `.mp3` files, then run `:tag`. All fields start
blank. Only fields the user fills in are written; blank fields are left
unchanged on each file. Useful for setting a shared album or artist
across multiple tracks. The Files box lists all selected filenames. Tab
completion tokens are drawn from all filenames combined.

## Commands

All commands are entered via the `:` command bar.

| Command          | Description                                        |
|------------------|----------------------------------------------------|
| `:convert`       | Convert selected file(s)/dir(s) from opus/m4a to mp3 |
| `:tag`           | Open ID3 tag editor for selected mp3 file(s)       |
| `:cd <path>`     | Change browser to an absolute or relative path     |
| `:cd`            | Change browser to the user's home directory        |
| `:q`             | Quit                                               |

### Tab completion

Tab completion works in two contexts:

**Command names** — with a bare word in the command bar, `Tab` cycles
through matching command names alphabetically. Each successive `Tab`
advances to the next match; the cycle wraps around. Any other keystroke
accepts the current completion and ends the cycle.

```text
:c<Tab>   → cd
:c<Tab>   → convert
:c<Tab>   → cd        (wraps)
```

**Directory paths** — after `cd` followed by a space, `Tab` completes
the path argument
using the longest common prefix of matching subdirectories. If exactly
one match exists, a trailing `/` is appended so the user can continue
tabbing deeper. Works with absolute paths, relative paths, and `~`.

## Error Handling

- If `ffmpeg` is not found on `$PATH`, the TUI opens normally and
  `:convert` shows an error in the status bar. Other features are
  unaffected.
- Conversion errors (e.g. corrupt source file) are shown in the status
  bar; conversion continues with the next file in the queue.
- Tagging errors (e.g. unreadable file) are shown in the status bar
  and return the user to the browser.
- Permission errors on directory reads are surfaced in the status bar;
  the browser shows an empty listing and navigation continues to work.
- Invalid paths supplied to `:cd` show `Not a directory: <path>` in
  the status bar.
- Unknown commands show `Unknown command: <name>` in the status bar.

## Future Considerations (out of scope for v1)

- Playback preview (e.g., via `mpv` or `ffplay`).
- FLAC/OGG conversion support.
- Album art embedding.
- Batch rename files from tag data.
