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
| `q` | Quit |
