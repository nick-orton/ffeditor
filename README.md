# ffeditor

A terminal UI tool for managing a digital music collection. Browse your 
filesystem, convert audio formats, and edit ID3 tags — all from the terminal.

## Requirements

- Go 1.22 or later

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
| `q` | Quit |

## Command Bar

Press `:` to open the command bar. Type a command and press `Enter` to execute, or `Esc` to cancel.

| Command | Description |
|---------|-------------|
| `:cd <path>` | Navigate to a directory |
| `:q` | Quit |

### `:cd` path syntax

- `:cd` — go to home directory
- `:cd ~` or `:cd ~/music` — `~` expands to your home directory
- `:cd /absolute/path` — absolute path
- `:cd relative/path` — relative to current directory
- Symlinks to directories are followed

### Tab completion

While typing a `:cd` command, press `Tab` to complete directory names:

- Completes to the longest common prefix of matching directories
- Appends `/` when there is exactly one match, so you can keep tabbing deeper
- Works with absolute paths, relative paths, and `~`
