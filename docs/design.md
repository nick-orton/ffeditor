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
- **Tag I/O:** [bogem/id3v2](https://github.com/bogem/id3v2)
  (MP3 ID3 tags), [go-flac/go-flac](https://github.com/go-flac/go-flac)
  + [go-flac/flacvorbis](https://github.com/go-flac/flacvorbis)
  (FLAC Vorbis Comments) — all pure Go, no CGo
- **Smart tag lookup:** Anthropic Messages API (`claude-haiku-4-5`);
  requires `ANTHROPIC_API_KEY` environment variable

## Architecture

```text
main.go              — entry point, arg parsing, Bubble Tea program init
model.go             — top-level TUI model, update loop, view
browser.go           — file-system browser component
converter.go         — audio conversion logic (ffmpeg wrapper)
tagger.go            — tag editor UI (format-agnostic)
tags.go              — tag I/O dispatch and format-specific backends
commands.go          — command-bar parsing & dispatch
claude.go            — Anthropic API call for smart tag lookup
```

## TUI Layout

```text
┌──────────────────────────────────────────────────────────────┐
│  FFEditor                             /home/user/Music       │ ← header
├──────────────────────────────────────────────────────────────┤
│  ▸ Albums/                                                   │
│    Tracks/                                                   │
│    cover.jpg                                                 │
│    song.opus                                                 │
│    demo.m4a                                                  │ ← browser
│    tagged.mp3          The Beatles · Come Together           │
│    album.flac          Pink Floyd · Breathe                  │
│    untagged.mp3        —                                     │
│    notes.txt                                                 │
├──────────────────────────────────────────────────────────────┤
│  [status / progress messages]                                │ ← status
├──────────────────────────────────────────────────────────────┤
│  > _                                                         │ ← command
└──────────────────────────────────────────────────────────────┘
```

### Navigation

| Key           | Action                                          |
|---------------|-------------------------------------------------|
| `↑` / `k`     | Move cursor up                                  |
| `↓` / `j`     | Move cursor down                                |
| `gg`          | Go to first entry                               |
| `G`           | Go to last entry                                |
| `Ctrl+U`      | Page up (half screen)                           |
| `Ctrl+D`      | Page down (half screen)                         |
| `Enter` / `l` | Enter directory or follow symlink to directory  |
| `h`           | Go to parent directory                          |
| `i`           | Toggle hidden files (dotfiles)                  |
| `Space`       | Toggle selection (for bulk ops), advance cursor |
| `Ctrl+A`      | Select all entries in current directory         |
| `e`           | Edit tags for selected `.mp3`/`.flac` file(s)   |
| `c`           | Convert selected audio files to `.mp3`          |
| `Ctrl+T`      | Fill missing tags (smart tags) for `.mp3`/`.flac`|
| `?`           | Show help screen (any key to dismiss)           |
| `:`           | Focus command bar                               |
| `Ctrl+C`      | Cancel in-progress conversion (stay in app)     |
| `q`           | Quit                                            |

## Features

### 1. File Browser

- Lists files and directories in the current path.
- Directories sort first, then files alphabetically
  (case-insensitive).
- Audio files (`.mp3`, `.opus`, `.m4a`, `.flac`, `.ogg`) are visually
  highlighted.
- Dotfiles (dot-prefixed names) are hidden by default. Press `i` to
  toggle them on; when visible they render dimmed.
- Symlinks are shown with a trailing `@` in cyan. Symlinks that point
  to a directory show `@/` and can be entered with `l` or `Enter`.
- Multi-select with `Space` for bulk operations, or `Ctrl+A` to
  select all entries at once; selection clears on directory change.
- When nothing is explicitly selected, commands operate on the entry
  under the cursor.
- Vim-style navigation: `gg` / `G` jump to the first/last entry;
  `Ctrl+U` / `Ctrl+D` scroll half a screen at a time.
- When pressing `h` to go to the parent directory, the cursor is
  restored to the subdirectory that was just left.
- Press `?` to open an in-app help screen listing all keybindings.
  Any key dismisses it.

#### Tag summary column

For taggable files (`.mp3` and `.flac`), the browser shows a tag
summary to the right of each filename. The summary displays
`Artist · Title`; if only one field is present, just that value is
shown. Files with no tag data show a dim `—` so untagged files are
immediately visible. Other audio files and non-audio entries show
nothing in this column.

The tag column is hidden automatically when the terminal is too narrow
to show it without obscuring the filename (minimum 12 characters of
available space required). Long summaries are truncated with `…`. The
cache is refreshed whenever the directory changes or tags are saved, so
the display stays current without a manual reload.

### 2. Audio Conversion

Converts audio files to `.mp3` by shelling out to `ffmpeg`.

Supported formats: `.opus`, `.ogg`, `.m4a`

#### Single file

Place the cursor on a file (or `Space`-select it) and run `:convert`.
The tool executes:

```text
ffmpeg -y -i input.opus -map_metadata:g 0:s:0 \
    -codec:a libmp3lame -qscale:a 2 output.mp3
```

- Output file is placed in the same directory with the `.mp3`
  extension.
- Source file is kept (not deleted).
- Metadata from the source file is copied into the output ID3 tag
  (see [Metadata copying](#metadata-copying) below).

#### Bulk convert

Select a directory (or multi-select files) and run `:convert`.

- Recursively finds all convertable audio files in the selection.
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

#### Metadata copying

When converting, the tool copies metadata from the source file into
the ID3 tag of the output `.mp3`. The six standard fields — Title,
Artist, Album, Year, Track, and Genre — are preserved where present.

Different container formats store tags at different levels, so the
`-map_metadata` flag passed to `ffmpeg` varies by extension:

| Format  | Tag storage                | ffmpeg flag                  |
|---------|----------------------------|------------------------------|
| `.m4a`  | Container atoms (global)   | `-map_metadata 0`            |
| `.opus` | Vorbis Comments (stream)   | `-map_metadata:g 0:s:0`      |
| `.ogg`  | Vorbis Comments (stream)   | `-map_metadata:g 0:s:0`      |

The `:g` specifier on the output side ensures all tags land in the
global (file-level) ID3 header rather than being attached to the
audio stream.

#### Cancellation

Press `Ctrl+C` during a conversion to kill the current ffmpeg process.
The application stays open, the status bar shows
`Conversion cancelled`, and the browser refreshes. Files already
converted before cancellation are kept.

### 3. Tag Editing

Select an `.mp3` or `.flac` file and press `e` (or run `:edit` /
`:tag`) to enter tag-editing mode. The editor supports both ID3v2 tags
(MP3) and Vorbis Comments (FLAC) transparently.

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

  Up/Down: navigate   Tab: complete   Ctrl+T: smart tags
  Ctrl+S: save   Esc: cancel
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
- `Ctrl+T` triggers smart tag lookup (single file only): sends the
  filename to Claude Haiku, which guesses Artist, Title, and Year.
  Blank fields are pre-filled with the result; non-blank fields are
  left unchanged. A `Searching...` spinner shows in the status bar
  while the model processes. Requires `ANTHROPIC_API_KEY`.
- `Ctrl+S` writes changed fields back to the file and returns to the
  browser.
- `Esc` discards changes and returns to the browser.

#### Bulk tagging

Multi-select several `.mp3` and/or `.flac` files, then press `e` (or
run `:edit`).
Fields shared by every selected file are pre-filled; differing fields
start blank. Only fields the user fills in are written; blank fields
are left unchanged on each file. The Title field is disabled in bulk
mode — it is shown dimmed and cannot receive focus — to prevent
accidentally overwriting individual track titles. Useful for setting
a shared album or artist across multiple tracks. The Files box lists
all selected filenames. Tab completion tokens are drawn from all
filenames combined.

### 4. Smart Tags from Browser

Press `Ctrl+T` in the file browser to fill missing tags for the
selected `.mp3` or `.flac` file(s) without opening the tag editor.
Works on a single file (cursor) or any number of space-selected files.

- For each file, the basename is sent to Claude Haiku, which guesses
  Artist, Title, and Year.
- Only fields that are currently **empty** are written back. Existing
  tag values are never overwritten.
- Files where Artist, Title, and Year are all already set are skipped
  entirely — no API call is made for them.
- A `Applying smart tags...` spinner blocks input during the operation.
- On completion, the status bar shows `Smart tags applied (N files)`
  and the tag summary column in the browser refreshes automatically.
- Requires `ANTHROPIC_API_KEY`. Shows an error if unset.

## Commands

All commands are entered via the `:` command bar. The most common
operations also have single-key shortcuts usable directly from the
browser without opening the command bar.

| Key / Command    | Description                                      |
|------------------|--------------------------------------------------|
| `e` / `:edit`    | Open tag editor for selected `.mp3`/`.flac` files |
| `c` / `:convert` | Convert selected file(s)/dir(s) to `.mp3`        |
| `:tag`           | Synonym for `:edit`                              |
| `:cd <path>`     | Change browser to an absolute or relative path   |
| `:cd`            | Change browser to the user's home directory      |
| `:q`             | Quit                                             |

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
:e<Tab>   → edit
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
- Album art embedding.
- Batch rename files from tag data.
