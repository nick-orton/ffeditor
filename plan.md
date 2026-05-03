# FFEditor — Phased Implementation Plan

## Context

FFEditor is a terminal UI music manager written in Go using Bubble Tea and 
Lip Gloss. It lets users browse their filesystem, convert opus/m4a files to mp3
via ffmpeg, and edit ID3 tags on mp3 files. This plan organizes implementation 
into phases where the app compiles and runs after each one, enabling 
incremental development.

**Canonical design references (must be consulted during each phase):**
- [architecture.md](./architecture.md) — struct layouts, message types, concurrency rules, key 
                    bindings, error handling, styling constants
- [design.md](./design.md) — TUI layout, feature descriptions, command table
- [requirements.md](./requirements.md) — high-level project goals

---

## File Layout (Final State)

```
ffeditor/
├── go.mod                 // module github.com/nick-orton/ffeditor
├── main.go
├── model.go               // Root model & message router (modified every phase)
├── browser.go             // File browser component
├── converter.go           // FFmpeg wrapper (Phase 3)
├── tagger.go              // ID3 tag editor (Phase 4)
├── commands.go            // Command bar (Phase 2)
├── styles.go              // Lip Gloss style constants
├── browser_test.go        // (Phase 6)
├── commands_test.go       // (Phase 6)
├── tagger_test.go         // (Phase 6)
├── converter_test.go      // (Phase 6)
└── testdata/              // small .opus fixture (Phase 6)
```

All files are `package main` — flat structure, no sub-packages.

---

## Phase 1 — Project Scaffold and File Browser

**Goal:** Working TUI with scrollable file browser, directory navigation, 
          four-zone layout. No commands processed yet. `q` quits.

### Files to Create

| File | What to implement |
|------|-------------------|
| `go.mod` | `module github.com/nick-orton/ffeditor`, Go 1.22, deps: `bubbletea`, `lipgloss`. Do NOT add `id3v2` yet. |
| `main.go` | Parse optional start-dir arg (default `os.Getwd()`). Probe ffmpeg via `exec.LookPath("ffmpeg")`. Call `tea.NewProgram(newModel(...), tea.WithAltScreen()).Run()`. |
| `styles.go` | All Lip Gloss style vars from architecture.md §6. No logic, just `var` declarations. |
j
| `browser.go` | Full `browserModel`: `readDir()`, dirs-first+alpha sort, viewport scrolling, `j`/`k`/arrows/`h`/Enter key handling, `selectedEntries()`, audio file detection (`isAudio`, `audioExts`). `dirChangedMsg` type. `View(width, height int)` with cursor, dir/audio/hidden styles. |
| `model.go` | `mode` type + constants (`modeBrowse`, `modeCommand`, `modeTag`). Full `model` struct with inline stubs for `cmdbarModel` and `taggerModel`. `newModel()`, `Init()`, `Update()` (routes keys to browser, handles `dirChangedMsg`, `tea.WindowSizeMsg`, `q`→quit). `View()` composes header + browser + status bar + static `"> "` prompt. |

**Stubs in model.go (removed in later phases):**
```go
type cmdbarModel struct{ input string; active bool }
type taggerModel  struct{ width int }
```

**Do NOT implement:** `:` activating command mode, `Space` multi-select, `converter.go`, `tagger.go`, `commands.go`.

### Verification
```
go build -o ffeditor .
./ffeditor /path/to/music
```
j/k/arrows move cursor, Enter enters dirs, `h` goes up, header updates path, `q` exits.

### architecture.md Sections
- §2.1 (startup) 
- §2.2 (root model + view composition) 
- §2.3 (browser struct, readDir, key table)
- §6 (styles).

---

## Phase 2 — Command Bar and `:cd` Command

**Goal:** 
- `:` enters command mode, command bar is editable. 
- `:cd <path>` navigates browser. 
- `Esc` cancels. 
- `Space` toggles multi-select.
- Unknown commands show error. 

### Files to Create/Modify

| File | Changes |
|------|---------|
| `commands.go` | Create. Full `cmdbarModel` (replaces stub in model.go). `parseCommand(input string) (cmd string, args []string)`. `View(width int)`. Key handling (printable appends, Backspace deletes, Esc clears). |
| `model.go` | Remove inline stubs. Wire `:` → `modeCommand`. Route keys in `modeCommand` to `cmdbar.Update()`. On Enter: call `parseCommand`, handle `"cd"` (validate dir, call `browser.changeDir()`), `"q"` (quit), unknown (error status). Wire `Space` to browser. |
| `browser.go` | Wire `Space` to toggle `selected[cursor]` and advance cursor. Render selected with `styleSelected`. |

**cd command details:**
- `:cd` with no arg → `os.UserHomeDir()`
- `:cd` with relative path → resolve via `filepath.Join(browser.dir, arg)` then `filepath.Abs`
- Invalid path → `statusMsg = "Not a directory: X"`, `statusIsError = true`

**Do NOT implement:** `:convert`, `:tag` (fall through to "Unknown command").

### Verification
`:` opens bar, Esc closes, `:cd /tmp` navigates, `:cd /bad` shows red error, 
    Space highlights entries.

### architecture.md Sections
- §2.6 (command bar, parseCommand, dispatch)
- §2.2 (mode switching, message routing) 
- §5 (error handling for cd)

---

## Phase 3 — Audio Conversion (`:convert`)

**Goal:** 
- `:convert` on selected opus/m4a files/dirs triggers ffmpeg via Bubble
  Tea Cmd goroutines. 
- Status shows `Converting N/M...`. Directory refreshes on completion.

### Files to Create/Modify

| File | Changes |
|------|---------|
| `converter.go` | Create. Message types: `convertDoneMsg`, `convertErrMsg`, `convertSkippedMsg`, `convertProgressMsg`. `convertFile(src string) tea.Cmd` — runs ffmpeg subprocess, sends message. `buildConvertList(entries []os.DirEntry, dir string) []string` — walks dirs via `filepath.WalkDir`, filters `.opus`/`.m4a`, deduplicates. |
| `model.go` | Add `convertQueue []string`, `convertIndex int`. Handle `execConvertMsg{files}`: validate ffmpeg available + non-empty list, set queue, return first `convertFile` Cmd. Handle `convertDoneMsg`/`convertErrMsg`/`convertSkippedMsg`: increment index, update status, chain next Cmd or finalize (re-read browser dir). |
| `commands.go` | Add `execConvertMsg{files []string}`. In Enter handler for `"convert"`: get `browser.selectedEntries()`, pass to `buildConvertList`, emit message. If ffmpeg unavailable, set error status immediately. |

**Sequential chain:**
```go
m.convertIndex++
if m.convertIndex < len(m.convertQueue) {
    return m, convertFile(m.convertQueue[m.convertIndex])
}
// Done: "Conversion complete (N converted, M skipped, E errors)"
// Re-read: browser.changeDir(browser.dir)
```

**Do NOT implement:** `:tag` (still unknown command), `tagger.go`.

### Verification
Navigate to dir with `.opus`, Space-select, `:convert` Enter → progress in 
status → `.mp3` appears in listing. Without ffmpeg: immediate error status.

### architecture.md Sections
- §2.4 (converter.go full spec, bulk flow steps 1–6) 
- §3.1 (single-file conversion flow) 
- §4 (concurrency — Cmds only, no locks) 
- §5 (ffmpeg error handling)

---

## Phase 4 — ID3 Tag Editing (`:tag`)

**Goal:** `:tag` on selected `.mp3` files enters `modeTag`. Browser region 
          replaced by tag form. Tab/Shift-Tab cycle fields, printable chars 
          edit, Enter saves, Esc cancels. Bulk: blank fields skip that tag.

### Files to Create/Modify

| File | Changes |
|------|---------|
| `tagger.go` | Create. `tagField` struct. `taggerModel` struct (replaces stub). `newTaggerModel(files []string)` — single file: pre-populate from `id3v2.Open`; multiple files: all blank. `Update()` — Tab/Shift-Tab (wrap), Enter (`saveTags` Cmd), Esc (`tagCancelledMsg`), printable/Backspace. `saveTags() tea.Cmd` — single: write dirty fields; bulk: write non-empty fields to all files. Returns `tagSavedMsg`, `tagBulkSavedMsg{count}`, `tagErrMsg`. `View(width, height int)` — renders 6-field form with hint line. |
| `model.go` | Remove `taggerModel` stub. Handle `execTagMsg{files}`: validate `.mp3` only, set `mode = modeTag`, `m.tagger = newTaggerModel(files)`. Route keys in `modeTag` to `tagger.Update()`. Handle `tagSavedMsg` → "Tags saved", `tagBulkSavedMsg` → "Tags updated (N files)", `tagCancelledMsg` → clear status; all return to `modeBrowse`. In `View()`: render tagger in browser region when `modeTag`. |
| `commands.go` | Add `execTagMsg{files}`. In Enter handler for `"tag"`: filter selection to `.mp3` only, emit message. If no `.mp3` in selection, error status. |
| `go.mod` | Add `github.com/bogem/id3v2/v2`. Run `go mod tidy`. |

**Fields:** Title, Artist, Album, Year, Track, Genre (6 total).

**id3v2 usage:**
```go
tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
defer tag.Close()
tag.SetTitle("New Title")
tag.Save()
```

**Do NOT implement:** year/track validation, playback.

### Verification
`:tag` on single `.mp3` → form with existing values. Tab cycles, typing edits, 
Enter saves. `:tag` on multiple `.mp3`s → blank form, fill one field, Enter 
writes only that field to all files.

### architecture.md Sections
- §2.5 (full taggerModel spec, id3v2 examples, single vs bulk, key table) 
- §3.2 (bulk tag flow)
- §6 (styleTagLabel, styleTagFocused).

---

## Phase 5 — Polish and Error Hardening

**Goal:** No new features. Cover all error paths from architecture.md §5 and 
          UX edge cases.

### Changes by File

**`main.go`:** Invalid start dir → print to stderr, exit 1 before TUI. ffmpeg 
               missing → initial `statusMsg = "ffmpeg not found — conversion disabled"` 
               (informational, not error color).

**`browser.go`:** Clamp cursor on dir change. `os.ReadDir` permission 
                  error → emit `dirReadErrMsg`, show empty listing. Hidden 
                  files (dot-prefix) → `lipgloss.NewStyle().Faint(true)`. 
                  Consistent `▸` prefix on cursor line.

**`converter.go`:** `buildConvertList` deduplicates paths. `convertSkippedMsg` 
                    tracked separately in summary: "N converted, M skipped, E 
                    errors".

**`tagger.go`:** `id3v2.Open` failure → return `tagErrMsg{err}`, root model 
                 sets error status, returns to `modeBrowse`. Always 
                 `defer tag.Close()`. Tab wraps last→first, Shift-Tab wraps 
                 first→last.

**`commands.go`:** `:cd` no arg → `os.UserHomeDir()`. `:cd` relative path → 
                   `filepath.Abs(filepath.Join(browser.dir, arg))`.

**`model.go`:** Handle `dirReadErrMsg` → set `statusMsg`/`statusIsError`. 
                Propagate `tea.WindowSizeMsg` to both `browser` and `tagger` 
                on every resize.

### Verification
- Invalid dir arg → clean error before TUI starts
- No ffmpeg → TUI opens, info message, `:convert` shows error
- Permission-denied dir → error in status, browser empty, nav still works
- Resize in tag mode → no visual corruption

### architecture.md Sections
- §5 (full error handling table) 
- §2.3 (hidden files, clamp) 
- §2.2 (WindowSizeMsg propagation).

---

## Phase 6 — Tests

**Goal:** `go test ./...` passes. ffmpeg tests skip when unavailable.

### Test Files

**`browser_test.go`**
- `TestReadDirSortOrder` — temp dir with subdirs + files, assert dirs-first 
                           then alpha
- `TestSelectedEntries_NoneSelected` — returns cursor entry when `selected` map
                                       empty
- `TestSelectedEntries_MultiSelected` — returns only toggled entries
- `TestScrolling` — cursor past viewport boundary adjusts offset

**`commands_test.go`**
- `TestParseCommand_Empty` — `""` → `"", nil`
- `TestParseCommand_NoArgs` — `"convert"` → `"convert", []`
- `TestParseCommand_WithArgs` — `"cd /home/user"` → `"cd", ["/home/user"]`
- `TestParseCommand_Whitespace` — `"  tag  "` → `"tag", []`

**`tagger_test.go`**
- `TestTagReadWrite` — write temp MP3, set title/artist, save, re-open, assert 
                       values match
- `TestBulkTag_BlankFieldSkipped` — two temp MPs with existing Artist, bulk-tag
                                    with blank Artist, assert original 
                                    unchanged

**`converter_test.go`** (integration, skips if ffmpeg absent)
- `TestConvertOpusToMp3` — copy `testdata/*.opus` fixture to temp dir, run 
                           `convertFile`, assert `.mp3` exists and non-zero
- `TestConvertSkipsExisting` — run twice, assert second returns 
                               `convertSkippedMsg`
- `TestBuildConvertList_Dedup` — entries with duplicates → no duplicates in 
                                 output

**Infrastructure:**
- Use `t.TempDir()` for all filesystem tests
- Add `testdata/` with minimal valid `.opus` fixture (few seconds of silence)

### Verification
```
go test ./...         # all unit tests pass
go test -short ./...  # skips integration tests
```

### architecture.md Sections
§9 (testing approach table — strategies map 1:1 to test cases above).

---

## Phase Dependency Summary

```
Phase 1: go.mod, main.go, styles.go, browser.go, model.go (stubs for cmd/tag)
Phase 2: commands.go (real), model.go (cmd routing + Space), browser.go (Space key)
Phase 3: converter.go, model.go (conversion queue + handlers)
Phase 4: tagger.go, model.go (tag routing), go.mod (id3v2 dep)
Phase 5: All files — hardening only, no new types
Phase 6: *_test.go files + testdata/
```

## Key Invariants (Maintain Throughout All Phases)

1. **`dirChangedMsg` contract:** Every function that changes `browser.dir` must
   emit `dirChangedMsg{path}` so the header stays in sync.
2. **Cmd discipline:** All file I/O (conversion, tag writes) runs in `tea.Cmd` 
   goroutines. `os.ReadDir` is synchronous in `Update` (fast for typical music 
   dirs). Never block `Update` with slow I/O.
3. **Stub replacement:** Phase 1 inline stubs in `model.go` are fully replaced 
   when their real files are created (Phase 2 for `cmdbarModel`, Phase 4 for 
   `taggerModel`).
