# FFEditor — Technical Architecture

[Design](./design.md)

## 1. System Overview

A terminal-based music collection manager built in Go using Bubble Tea. The
application follows a component-oriented architecture where each UI concern
(browser, tagger, command bar) is encapsulated as its own Bubble Tea model,
composed under a single root model that manages focus and message routing.

```
                  ┌────────────────┐
                  │   main.go      │
                  │  arg parsing   │
                  │  tea.NewProgram│
                  └──────┬─────────┘
                         │
                  ┌──────▼───────┐
                  │  model.go    │
                  │  root model  │
                  │  (router)    │
                  └──┬───┬───┬─ ─┘
           ┌─────────┘   │   └─────────┐
     ┌─────▼──────┐ ┌─────▼─────┐ ┌────▼────────┐
     │ browser.go │ │ tagger.go │ │commands.go  │
     │ file list  │ │ tag editor│ │ cmd bar     │
     └─────┬──────┘ └─────┬─────┘ └───┬─────────┘
           │              │           │
           │        ┌─────▼─────┐     │
           │        │  id3 lib  │     │
           │        └───────────┘     │
           │                          │
     ┌─────▼──────────────────────────▼──┐
     │          converter.go             │
     │     ffmpeg subprocess mgmt        │
     └───────────────────────────────────┘
```

## 2. Module Specifications

### 2.1 `main.go` — Entry Point

**Responsibilities:**
- Parse CLI arguments (optional starting directory, defaults to `cwd`).
- Validate that `ffmpeg` exists on `$PATH` using `exec.LookPath("ffmpeg")`.
  Store the result as a boolean `ffmpegAvailable` on the root model.
- Initialize the root model and start `tea.NewProgram` with
  `tea.WithAltScreen()` for full-screen mode.

**Startup sequence:**
```
1. Parse args → resolve starting directory to absolute path
2. Probe ffmpeg → set ffmpegAvailable flag
3. Read initial directory listing → populate browser state
4. tea.NewProgram(newModel(dir, ffmpegAvailable)).Run()
```

### 2.2 `model.go` — Root Model & Message Router

**Struct:**
```go
type mode int

const (
    modeBrowse mode = iota
    modeCommand
    modeTag
)

type model struct {
    mode             mode
    width, height    int             // terminal dimensions from tea.WindowSizeMsg
    browser          browserModel
    tagger           taggerModel
    cmdbar           cmdbarModel
    statusMsg        string          // current status bar text
    statusIsError    bool            // render status in error style
    ffmpegAvailable  bool
    convertQueue     []string        // files pending conversion
    convertIndex     int             // index of the file currently being converted
    convertDone      int             // files successfully converted
    convertSkipped   int             // files skipped (output already exists)
    convertErrors    int             // files that failed
    convertCtx       context.Context // cancelled to kill the active ffmpeg process
    convertCancel    context.CancelFunc
    convertCancelled bool            // set when Ctrl+C aborts the queue
}
```

**Message routing rules:**

| `m.mode`      | Key messages routed to | Notes                                        |
|---------------|------------------------|----------------------------------------------|
| `modeBrowse`  | `browser`              | `:` switches to `modeCommand`; `Ctrl+C` cancels conversion; `q` quits |
| `modeCommand` | `cmdbar`               | `Enter` dispatches, `Tab` completes, `Esc` exits |
| `modeTag`     | `tagger`               | `Enter` saves, `Esc` cancels                 |

All modes receive `tea.WindowSizeMsg` for responsive layout. Custom messages
(conversion progress, completion, errors) are handled at the root level to
update `statusMsg`.

**`Ctrl+C` handling in `modeBrowse`:**
- If `convertCancel != nil` (conversion in progress): call `convertCancel()`,
  set `convertCancelled = true`, set `statusMsg = "Conversion cancelled"`.
  The app stays open; the in-flight `convertFile` goroutine returns a
  `convertErrMsg` which `nextConvert` detects via the `convertCancelled` flag
  and discards without chaining the next file.
- If no conversion is in progress: no-op (`q` is the quit key).

**View composition:**
```
header      → app title + current path (1 line)
browser     → height - 4 lines (fills remaining space)
  OR tagger → same region when mode == modeTag
status bar  → 1 line
command bar → 1 line (visible in all modes, editable in modeCommand)
```

The root `View()` uses Lip Gloss `JoinVertical` to stack these sections. Each
sub-model's `View()` receives available width/height so it can render
correctly.

### 2.3 `browser.go` — File System Browser

**Struct:**
```go
type browserModel struct {
    dir       string        // current absolute directory path
    entries   []os.DirEntry // current listing
    cursor    int           // index of highlighted entry
    offset    int           // scroll offset for viewport
    selected  map[int]bool  // indices toggled with Space
    height    int           // visible rows (set by parent)
}
```

**Directory reading:**
- Use `os.ReadDir(dir)` which returns entries sorted by name.
- Post-sort: directories first (stable sort by `IsDir()` descending), then
  files alphabetically. Hidden files (dot-prefix) are included but rendered
  dimmer.

**Scrolling:**
- Viewport window: `[offset, offset+height)`.
- When `cursor` moves outside the viewport, adjust `offset` to keep cursor
  visible (scroll by 1 line, no page jumping).

**Selection:**
- `Space` toggles `selected[cursor]`.
- Selection is cleared on directory change.
- `selectedEntries()` returns `[]os.DirEntry` of toggled items. If nothing is
  toggled, returns a slice containing only the cursor entry — this provides
  unified handling for single vs. multi operations.

**Audio file detection:**
```go
var audioExts = map[string]bool{
    ".mp3": true, ".opus": true, ".m4a": true,
    ".flac": true, ".ogg": true,
}

func isAudio(name string) bool {
    return audioExts[strings.ToLower(filepath.Ext(name))]
}
```

Audio files are rendered with a distinct Lip Gloss style (e.g., cyan
foreground). Directories get a trailing `/` and bold style. Selected entries
get an inverted/highlighted background.

**Key handling:**

| Key         | Action                                              |
|-------------|-----------------------------------------------------|
| `j` / `Down`| `cursor++` (clamp to len-1)                        |
| `k` / `Up`  | `cursor--` (clamp to 0)                            |
| `Enter`     | If dir: `cd` into it. If file: no-op (selected for commands) |
| `h`         | `cd` to parent (`filepath.Dir(dir)`)                |
| `Space`     | Toggle `selected[cursor]`, advance cursor           |

On directory change, emit a custom `dirChangedMsg{path}` so the root model
can update the header.

### 2.4 `converter.go` — FFmpeg Wrapper

This module contains no TUI code. It exposes pure functions and returns
Bubble Tea `Cmd`s for async execution.

**Core function:**
```go
func convertFile(ctx context.Context, src, destDir string) tea.Cmd {
    return func() tea.Msg {
        dest := filepath.Join(destDir,
            strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))+".mp3")

        if _, err := os.Stat(dest); err == nil {
            return convertSkippedMsg{src}
        }

        cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", src,
            "-codec:a", "libmp3lame", "-qscale:a", "2", dest)
        cmd.Stdout = nil
        cmd.Stderr = nil

        if err := cmd.Run(); err != nil {
            return convertErrMsg{src, err}
        }
        return convertDoneMsg{src, dest}
    }
}
```

The `context.Context` parameter is used for cancellation: when the context is
cancelled (e.g. via `Ctrl+C`), `exec.CommandContext` kills the ffmpeg process
immediately.

**Message types:**
```go
type convertDoneMsg     struct{ src, dest string }
type convertErrMsg      struct{ src string; err error }
type convertSkippedMsg  struct{ src string }
type convertProgressMsg struct{ current, total int }
```

**Bulk conversion flow:**

1. Root model receives `execConvertMsg{files}`.
2. Validate ffmpeg is available and the file list is non-empty.
3. Create a cancellable context and store it on the model:
   ```go
   ctx, cancel := context.WithCancel(context.Background())
   m.convertCtx = ctx
   m.convertCancel = cancel
   ```
4. Store the file list and reset counters:
   ```go
   m.convertQueue = files
   m.convertIndex = 0
   m.convertDone, m.convertSkipped, m.convertErrors = 0, 0, 0
   ```
5. Return `convertFile(ctx, files[0], dir)` as the first `Cmd`.
6. On each `convertDoneMsg`/`convertErrMsg`/`convertSkippedMsg`, increment
   the relevant counter and call `nextConvert`:
   - If `convertCancelled`: clean up context, clear the queue, refresh
     browser dir, return nil (no further Cmds).
   - If `convertIndex < len(convertQueue)`: update status to
     `"Converting N/M..."`, return the next `convertFile` Cmd.
   - Otherwise: call `convertCancel()`, set final status
     `"Conversion complete (N converted, M skipped, E errors)"`,
     refresh browser dir.

Conversions run **sequentially** (one ffmpeg process at a time) to avoid
saturating CPU/disk on large batches.

### 2.5 `tagger.go` — ID3 Tag Editor

**Struct:**
```go
type taggerModel struct {
    files      []string   // files being edited
    fields     []tagField // ordered: title, artist, album, year, track, genre
    focusIndex int        // which field has cursor
    width      int
    tokens     []string   // tokens parsed from filename(s) for tab completion
    tabStem    string     // field value before the word being completed
    tabPrefix  string     // the word prefix when the tab cycle started
    tabMatches []string   // candidates for current cycle
    tabIndex   int        // next index within tabMatches
}

type tagField struct {
    label    string // "Title", "Artist", etc.
    value    string // editable text
    original string // value loaded from file (for dirty check)
}
```

**Library choice: `github.com/bogem/id3v2`**

Pure Go, no CGo dependency. Supports ID3v2.3 and ID3v2.4 frames. Usage:

```go
// Read
tag, _ := id3v2.Open(path, id3v2.Options{Parse: true})
title := tag.Title()
artist := tag.Artist()
tag.Close()

// Write
tag, _ := id3v2.Open(path, id3v2.Options{Parse: true})
tag.SetTitle("New Title")
tag.Save()
tag.Close()

// Track number (TRCK frame — no dedicated method)
frame := tag.GetLastFrame("TRCK")
if tf, ok := frame.(id3v2.TextFrame); ok { track = tf.Text }
tag.DeleteFrames("TRCK")
tag.AddTextFrame("TRCK", id3v2.EncodingUTF8, value)
```

**Single-file flow:**
1. Open the file, read all six fields into `tagField` structs with `original`
   set.
2. Render the tag editing view (see design.md layout).
3. On `Ctrl+S`: for each field where `value != original`, write the new value.
   Close the tag handle. Return `tagSavedMsg`.
4. On `Esc`: discard, return `tagCancelledMsg`.

**Bulk tagging flow:**
1. All six fields start blank (empty `value` and empty `original`).
2. On `Ctrl+S`: for each selected file, open the tag, and for each field where
   `value != ""`, overwrite that field. Fields left blank are untouched.
3. Return `tagBulkSavedMsg{count}`.

**Key handling in tag mode:**

| Key          | Action                                               |
|--------------|------------------------------------------------------|
| `↑` / `Shift+Tab` | `focusIndex = (focusIndex+5) % 6` (wrap up)   |
| `↓`          | `focusIndex = (focusIndex+1) % 6` (wrap down)        |
| `Tab`        | Complete current word from filename tokens (cycle)   |
| `Ctrl+S`     | Save and return to browser                           |
| `Esc`        | Cancel and return to browser                         |
| Printable    | Append to `fields[focusIndex].value`; resets tab cycle|
| `Backspace`  | Delete last char from focused field; resets tab cycle|

**Tab completion:**

`tokenizeFilenames(files []string) []string` splits each filename (extension
stripped) on non-alphanumeric characters and returns a deduplicated token list
in order of first appearance. For bulk edits, tokens from all filenames are
combined.

`handleTab()` implements prefix completion on the *current word* (the text
after the last space in the focused field):
1. On first `Tab`: split field value into `tabStem` (text up to and including
   the last space) and `tabPrefix` (the word being typed). Collect all tokens
   where `strings.HasPrefix(lower(tok), lower(tabPrefix))` into `tabMatches`.
2. Set `fields[focusIndex].value = tabStem + tabMatches[tabIndex]`, advance
   `tabIndex` (wrapping).
3. Subsequent `Tab` presses cycle through `tabMatches`.
4. Any printable character, `Backspace`, or navigation key resets `tabMatches = nil`.

**View:**

`View(width, height int)` renders two titled rounded boxes (color 62) stacked
vertically, centered in the available height:

```
╭─ Files ─────────────────────────────────────╮
│ song.mp3                                    │
╰─────────────────────────────────────────────╯

╭─ Tags ──────────────────────────────────────╮
│     Title: My Song▌                         │
│    Artist:                                  │
│     Album:                                  │
│      Year:                                  │
│     Track:                                  │
│     Genre:                                  │
╰──────────────────────────────��──────────────╯

  Up/Down: navigate   Tab: complete   Ctrl+S: save   Esc: cancel
```

Boxes are drawn by `titledBox(title, content string, width int) string`, which
manually constructs the rounded border lines since lipgloss v1.1.0 does not
expose a border-title API. The focused field's value is rendered with
`styleTagFocused` (underline) and a `▌` cursor appended.

### 2.6 `commands.go` — Command Bar

**Struct:**
```go
type cmdbarModel struct {
    input      string   // raw text after ":"
    active     bool     // whether the bar is focused
    tabPrefix  string   // input prefix when the current tab cycle started
    tabMatches []string // nil when no active tab cycle
    tabIndex   int      // next index to use within tabMatches
}
```

**Parsing:**
Split `input` on whitespace. `args[0]` is the command name, `args[1:]` are
arguments. No shell-style quoting is needed for v1.

```go
func parseCommand(input string) (cmd string, args []string) {
    parts := strings.Fields(input)
    if len(parts) == 0 {
        return "", nil
    }
    return parts[0], parts[1:]
}
```

**Tab completion:**

Tab completion is handled by `handleTab(browserDir string) cmdbarModel`, called
from the root model when `Tab` is pressed in `modeCommand`.

*Command-name cycling* — when the input contains no space (bare word):
1. On the first `Tab`, record `tabPrefix = input` and collect all entries from
   `knownCommands` (`"cd"`, `"convert"`, `"q"`, `"tag"`) that start with the
   prefix, in alphabetical order, into `tabMatches`.
2. Set `input = tabMatches[tabIndex]` and advance `tabIndex` (wrapping).
3. Subsequent `Tab` presses cycle through the same `tabMatches` list.
4. Any non-`Tab` key (printable character, `Backspace`, `Esc`) resets
   `tabMatches = nil`, ending the cycle.

*Path completion* — when the input starts with `cd ` (has a space):
- `tabMatches` is cleared and `tabCompletePath` is called, which resolves the
  partial path argument and completes to the longest common prefix of matching
  subdirectories. Appends `/` when there is exactly one match. No cycling.

**Dispatch table** (handled in `model.go`'s `dispatchCommand`):

| Command    | Validation                                                          | Behaviour                                      |
|------------|---------------------------------------------------------------------|------------------------------------------------|
| `convert`  | `ffmpegAvailable` must be true; selection must contain convertible files | Returns a Cmd emitting `execConvertMsg{files}` |
| `tag`      | Selection must contain `.mp3` files                                 | Returns a Cmd emitting `execTagMsg{files}`     |
| `cd`       | Path must resolve to an existing directory                          | Calls `browser.changeDir` directly             |
| `q`        | —                                                                   | Returns `tea.Quit`                             |

Unknown commands → set `statusMsg` to `"Unknown command: foo"`.

On `Enter`, the command bar parses, dispatches via `dispatchCommand`, clears
`input`, sets `active = false`, and returns focus to `modeBrowse`.

## 3. Message Flow Diagrams

### 3.1 Single File Conversion

```
User presses ':'          → mode = modeCommand
User types 'convert'      → cmdbar.input = "convert"
User presses Enter        → parseCommand → dispatchCommand
                           → buildConvertList → execConvertMsg{files} Cmd
Root.Update receives       → create context/cancel
  execConvertMsg           → set statusMsg = "Converting 1/1..."
                           → return convertFile(ctx, src, dir) Cmd
tea runtime calls Cmd      → ffmpeg runs in goroutine
ffmpeg completes           → convertDoneMsg{src, dest}
Root.Update receives       → nextConvert: queue exhausted
  convertDoneMsg           → statusMsg = "Conversion complete (...)"
                           → browser.changeDir → entries refreshed
```

### 3.2 Bulk Tag Edit

```
User selects files         → Space on each → browser.selected = {0,2,5}
User presses ':'           → mode = modeCommand
User types 'tag'           → cmdbar.input = "tag"
User presses Enter         → execTagMsg{files}
Root.Update receives       → mode = modeTag
  execTagMsg                → tagger = newTaggerModel(files) (blank fields)
User fills in Artist       → tagger.fields[1].value = "New Artist"
User presses Enter         → write Artist to all 3 files
                            → tagBulkSavedMsg{3}
Root.Update receives       → mode = modeBrowse
  tagBulkSavedMsg           → statusMsg = "Tags updated (3 files)"
```

### 3.3 Conversion Cancellation

```
Conversion in progress     → convertFile goroutine running ffmpeg
User presses Ctrl+C        → convertCancel() called
                           → convertCancelled = true
                           → statusMsg = "Conversion cancelled"
                           → app stays open (no tea.Quit)
ffmpeg process killed      → convertFile goroutine returns convertErrMsg
Root.Update receives       → nextConvert checks convertCancelled == true
  convertErrMsg            → discards remaining queue
                           → clears context/cancel/cancelled fields
                           → browser.changeDir → entries refreshed
```

## 4. Concurrency Model

Bubble Tea handles concurrency via `Cmd` functions — each `Cmd` runs in its
own goroutine managed by the tea runtime. The application itself is
single-threaded from the perspective of `Update`: messages arrive
sequentially, and no locks are needed on model state.

**Rules:**
- File I/O (conversion, tag writing) always happens inside a `Cmd`, never
  in `Update` directly.
- Directory reads (`os.ReadDir`) are fast enough to run synchronously in
  `Update` for typical music directories (< 10k entries).
- Only one conversion Cmd is in-flight at a time (sequential queue).
- Cancellation is coordinated via `context.WithCancel`: calling
  `convertCancel()` causes `exec.CommandContext` to kill the ffmpeg process,
  and the goroutine returns a `convertErrMsg` which the root model uses to
  detect the cancellation and stop the queue.

## 5. Error Handling Strategy

| Error class              | Detection point        | User-facing behavior                                      |
|--------------------------|------------------------|-----------------------------------------------------------|
| ffmpeg not on PATH       | `main.go` startup      | `ffmpegAvailable = false`; `:convert` shows error status  |
| ffmpeg process failure   | `convertFile` Cmd      | `convertErrMsg` → status bar, continue next file          |
| Conversion cancelled     | `Ctrl+C` in browse mode| ffmpeg killed via context; status "Conversion cancelled"; app stays open |
| File permission denied   | `os.ReadDir`, tag I/O  | Status bar error message                                  |
| Invalid tag file         | `id3v2.Open`           | Status bar error, return to browser                       |
| Unknown command          | `dispatchCommand`      | Status bar: "Unknown command: X"                          |
| cd to non-existent dir   | `dispatchCommand`      | Status bar: "Not a directory: X"                          |

Errors never cause a panic or program exit. All errors are surfaced through
`statusMsg` with `statusIsError = true` (rendered in red).

## 6. Styling (Lip Gloss)

Define a central `styles.go` (or a `var` block in `model.go`) with all
style constants:

```go
var (
    styleHeader     = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("62"))
    styleDir        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
    styleAudio      = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
    styleCursor     = lipgloss.NewStyle().Background(lipgloss.Color("237"))
    styleSelected   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
    styleStatusOk   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    styleStatusErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
    styleTagLabel   = lipgloss.NewStyle().Width(10).Align(lipgloss.Right)
    styleTagFocused = lipgloss.NewStyle().Underline(true)
    styleCmdPrefix  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
```

## 7. Package & Dependency Layout

```
ffeditor/
├── go.mod                 // module github.com/nick-orton/ffeditor
├── go.sum
├── main.go
├── model.go
├── browser.go
├── converter.go
├── tagger.go
├── commands.go
└── styles.go              // optional, can be inlined
```

**Dependencies (`go.mod`):**
```
require (
    github.com/charmbracelet/bubbletea  v1.x
    github.com/charmbracelet/lipgloss   v1.x
    github.com/bogem/id3v2/v2           v2.x
)
```

All source files are in `package main`. No internal packages for v1 — the
application is small enough that a flat structure keeps navigation simple.
Extracting packages (e.g., `pkg/convert`, `pkg/tag`) is warranted only if the
codebase grows substantially.

## 8. Build & Runtime Requirements

- **Go >= 1.22**
- **ffmpeg** on `$PATH` (optional — app runs without it but disables conversion)
- No CGo required (pure Go ID3 library)
- Build: `go build -o ffeditor .`
- Run: `./ffeditor [starting-directory]`

## 9. Testing Approach

| Layer        | Strategy                                                 |
|--------------|----------------------------------------------------------|
| `converter`  | Integration test with a small `.opus` fixture; verify `.mp3` output exists and is valid audio. Skip if ffmpeg not available (`testing.Short`). |
| `tagger`     | Unit test: write known tags to a temp `.mp3`, read back, assert equality. Pure Go, no external deps. |
| `browser`    | Unit test: create a temp directory tree, call `readDir`, assert sort order (dirs first, alpha). |
| `commands`   | Unit test: `parseCommand` with various inputs, assert command name and args. Unit test `handleTab` cycling and reset behaviour. |
| TUI          | Manual testing. Bubble Tea's `tea.Test` helpers can be used for simple smoke tests (send keys, assert final model state). |
