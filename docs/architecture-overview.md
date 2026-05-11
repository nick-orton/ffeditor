# Architecture Overview

## Approach

FFEditor is a flat, single-package Go TUI application (`package main`,
all files in the root directory). The flat package structure is 
intentional; do not introduce sub-packages unless the codebase grows 
substantially.

The architecture follows Bubble Tea's component model: `model.go` owns a 
mode state machine and routes messages to three sub-components — 
`browser.go`, `tagger.go`, and `commands.go` — each a self-contained 
Bubble Tea model with its own `Update` and `View`. 

Side-effecting work (file I/O, ffmpeg subprocesses, API calls) is always 
lifted into `tea.Cmd` goroutines, never run directly in `Update`. 

New features should follow the same pattern: add a sub-model file for any 
new UI concern, expose pure `tea.Cmd` factory functions for any new I/O, 
define message types in the same file as the code that produces them, and 
route them through `model.go`. All styles belong in `styles.go`. 


## Source Files

| File            | Purpose                                              |
|-----------------|------------------------------------------------------|
| `main.go`       | Entry point: arg parsing, ffmpeg probe, program init |
| `model.go`      | Root Bubble Tea model; mode state machine and router |
| `keys.go`       | Key event dispatch; per-mode key handler functions   |
| `browser.go`    | File system browser component                        |
| `converter.go`  | FFmpeg subprocess wrapper; bulk conversion logic     |
| `tagger.go`     | Tag editor component (single and bulk; format-agnostic) |
| `tags.go`       | Tag I/O dispatch; MP3 (ID3) and FLAC (Vorbis) backends |
| `formats.go`    | Audio extension sets (`audioExts`, `convertibleExts`, `blessedExts`) and predicates |
| `commands.go`   | Command bar component; command parsing and dispatch  |
| `claude.go`     | Smart tag lookup via the Anthropic Messages API      |
| `help.go`       | Help screen view rendering                           |
| `styles.go`     | All Lip Gloss style constants                        |

## Test Files

| File                | Tests for     |
|---------------------|---------------|
| `browser_test.go`   | `browser.go`  |
| `commands_test.go`  | `commands.go` |
| `converter_test.go` | `converter.go`|
| `tagger_test.go`    | `tagger.go`   |
| `claude_test.go`    | `claude.go`   |

## Where to Find More Detail

For full detail on each module, see [architecture.md](./architecture.md).

- **`main.go`** — startup sequence: §2.1
- **`model.go`** — mode enum, struct, message routing table,
  view composition: §2.2
- **`keys.go`** — key routing and per-mode handlers: §2.2 (routing
  table), §2.3–2.6 (per-mode key tables)
- **`browser.go`** — directory reading, sorting, symlink handling,
  tag cache, selection, key bindings: §2.3
- **`converter.go`** — `convertFile` implementation, message types,
  bulk conversion flow, cancellation: §2.4
- **`tagger.go`** — single-file and bulk tag flows, tab completion,
  view layout: §2.5
- **`tags.go`** — `readTags`/`writeTags` dispatch, MP3 and FLAC
  backends, Vorbis Comment key mapping: §2.6
- **`commands.go`** — command parsing, tab completion, dispatch
  table: §2.7
- **`claude.go`** — API request/response flow, test overrides: §2.8
- **`formats.go`** — extension sets and predicates: §2.3 (audio
  file detection)
- **`styles.go`** — full style listing: §6
- **Message flow diagrams** (conversion, bulk tag, smart tag,
  cancellation): §3
- **Concurrency model**: §4
- **Error handling**: §5
- **Testing approach**: §9
