# ffeditor

A terminal UI tool for managing a digital music collection. Browse your 
filesystem, convert audio formats, and edit ID3 tags — all from the terminal.

## Requirements

- Go 1.22 or later
- `ffmpeg` on `$PATH` (required for audio conversion)

## Build

```sh
git clone https://github.com/nick-orton/ffeditor
cd ffeditor
go build -o ffeditor .
```

## Usage

```sh
./ffeditor [directory]
```

Opens the file browser in the given directory, or the current working directory 
if none is provided.

## Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `l` / `Enter` | Enter directory |
| `h` | Go to parent directory |
| `Space` | Toggle selection (advances cursor) |
| `Ctrl+C` | Cancel in-progress conversion |
| `q` | Quit |

## Command Bar

Press `:` to open the command bar. Type a command and press `Enter` to execute, or `Esc` to cancel.

| Command | Description |
|---------|-------------|
| `:convert` | Convert selected `.opus`/`.m4a` files to `.mp3` |
| `:cd <path>` | Navigate to a directory |
| `:q` | Quit |

### Tab completion

Tab completion works in two contexts:

**Command names** — press `Tab` with a partial command name to cycle through matching commands alphabetically:

```
:c<Tab>      → :cd
:c<Tab>      → :convert
:c<Tab>      → :cd          (wraps around)
```

Any other keystroke accepts the current completion and ends the cycle.

**Directory paths** — while typing a `:cd` command, press `Tab` to complete directory names:

- Completes to the longest common prefix of matching directories
- Appends `/` when there is exactly one match, so you can keep tabbing deeper
- Works with absolute paths, relative paths, and `~`

### `:cd` path syntax

- `:cd` — go to home directory
- `:cd ~` or `:cd ~/music` — `~` expands to your home directory
- `:cd /absolute/path` — absolute path
- `:cd relative/path` — relative to current directory

## Audio Conversion

Select one or more `.opus` or `.m4a` files (or a directory containing them) and run `:convert`. Converted `.mp3` files are written alongside the originals. Source files are not deleted.

- **Bulk**: selecting a directory recursively finds all convertible files and converts them sequentially, showing `Converting N/M...` progress in the status bar.
- **Skip**: files that already have a corresponding `.mp3` are skipped automatically.
- **Cancel**: press `Ctrl+C` during a conversion to kill the current ffmpeg process and stop the queue. Files already converted are kept. The browser returns to normal immediately.
- **No ffmpeg**: if `ffmpeg` is not found on `$PATH`, the app opens normally and `:convert` shows an error in the status bar.
