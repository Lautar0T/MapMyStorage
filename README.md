# real-disk-map

`real-disk-map` is a cross-platform CLI for finding what is actually using disk space.
It reports allocated size, also known as physical size or size on disk, instead of only logical file size.

This is useful for inspecting sparse files, compressed files, and cloud-storage placeholders that can appear huge logically while using little or no local disk space.

## Features

- Interactive terminal UI for drilling into folders.
- JSON and CSV output for scripts and reports.
- Physical and logical size side by side.
- Optional hidden files, symlink following, max depth, and exclude patterns.
- macOS, Linux, and Windows builds.
- Cloud placeholder hints for iCloud, Dropbox, OneDrive, and Google Drive where filesystem metadata is available.

## Install

Download a binary from the [latest GitHub release](https://github.com/lautaro/real-disk-map/releases/latest).

macOS and Linux can also use the install script:

```sh
curl -fsSL https://raw.githubusercontent.com/lautaro/real-disk-map/main/scripts/install.sh | sh
```

Or install from source with Go:

```sh
go install github.com/lautaro/real-disk-map/cmd/rdm-cli@latest
```

Go 1.25 or newer is required.

## Usage

Start the interactive TUI:

```sh
rdm-cli
rdm-cli ~/Downloads
```

Export JSON or CSV:

```sh
rdm-cli -json -root ~/Downloads > disk.json
rdm-cli -csv -root ~/Downloads > disk.csv
```

Run a non-interactive text summary:

```sh
rdm-cli -no-interactive -root ~/Downloads
```

Filter scans:

```sh
rdm-cli -exclude ".git,node_modules,*.tmp" -hidden -max-depth 4
```

## Options

```text
-version            Show version
-help               Show help
-root string        Root path to scan (default: home)
-json               Output full JSON tree
-csv                Output flattened CSV
-no-interactive     Text summary mode
-hidden             Show hidden files
-follow-symlinks    Follow symbolic links (default: false)
-max-depth int      Max depth (0 unlimited)
-exclude string     Comma-separated exclude patterns
```

## TUI Shortcuts

```text
arrows              move
Enter               enter directory / reveal file
Backspace / Left    go to parent
r                   refresh
s                   cycle sort
/                   search
h                   toggle hidden
l                   toggle real/logical columns
v                   visual summary panel
o                   open/reveal in Finder/Explorer
t                   open in terminal
y                   copy path
e                   export JSON/CSV
q                   quit
```

## Physical vs Logical Size

Logical size is the size reported by file metadata. Physical size is the amount of local disk allocation consumed by the file.

Examples:

- A sparse file may have a large logical size but only allocate a few blocks.
- A compressed file may allocate less disk space than its logical size.
- A cloud placeholder can report the remote file's logical size while using little local storage.

`real-disk-map` ranks by physical size by default so the biggest entries are the ones actually consuming local disk.

## Build From Source

```sh
git clone https://github.com/lautaro/real-disk-map.git
cd real-disk-map
make build
./bin/rdm-cli -version
```

Useful development commands:

```sh
make test
make vet
make build-all
```

## Platform Notes

- macOS uses `st_blocks * 512` and extended attributes for cloud hints when available.
- Linux uses allocated block metadata from `stat`.
- Windows uses Win32 APIs including `GetCompressedFileSizeW` and filesystem cluster size.
- Cloud provider metadata varies by provider, OS version, filesystem, and sync client configuration, so cloud status should be treated as a best-effort hint.

## License

MIT
