# MapMyStorage

`MapMyStorage` is a cross-platform CLI for finding what is actually using disk space.
It reports allocated size, also known as size on disk, instead of only logical file size.

This is useful for inspecting sparse files, compressed files, and cloud-storage placeholders that can appear huge logically while using little or no local disk space.

## Features

- Interactive terminal UI for drilling into folders.
- JSON and CSV output for scripts and reports.
- Physical and logical size side by side.
- Hidden files by default, optional symlink following, max depth, and exclude patterns.
- macOS, Linux, and Windows builds.
- Cloud placeholder hints for iCloud, Dropbox, OneDrive, and Google Drive where filesystem metadata is available.

## Install

Download a binary from the [latest GitHub release](https://github.com/lautar0t/MapMyStorage/releases/latest).

macOS and Linux can also use the install script:

```sh
curl -fsSL https://raw.githubusercontent.com/lautar0t/MapMyStorage/main/scripts/install.sh | sh
```

Or install from source with Go:

```sh
go install github.com/lautar0t/MapMyStorage/cmd/mapmystorage@latest
```

Go 1.25 or newer is required. Release downloads and `@latest` refer to the latest
published tag; to install the changes currently on `main`, use:

```sh
go install github.com/lautar0t/MapMyStorage/cmd/mapmystorage@main
```

From a local checkout, `make install` builds and installs to `~/.local/bin`.
Ensure that directory is in your `PATH`. Override with
`make install INSTALL_DIR=/your/bin` if needed. `make uninstall` uses the same
installation directory. `make build` only writes the local `bin/` executable.

## Usage

Start the interactive TUI:

```sh
mapmystorage
mapmystorage ~/Downloads
```

Export JSON or CSV:

```sh
mapmystorage -json -root ~/Downloads > disk.json
mapmystorage -csv -root ~/Downloads > disk.csv
```

Run a non-interactive text summary:

```sh
mapmystorage -no-interactive -root ~/Downloads
```

Filter scans:

```sh
mapmystorage -exclude ".git,node_modules,*.tmp" -hidden -max-depth 4
```

Both `--option` and `-option` work. `mapmystorage --help` (also `-h`) includes
examples, flags, coverage limits, shortcuts and exit codes. Put options before
the positional path; extra paths and conflicting roots are rejected. Piped or
redirected input/output selects text mode automatically; JSON/CSV remain explicit.
An exit code of 0 can still mean a partial scan: inspect coverage. Runtime errors
exit 1 and invalid arguments exit 2.

## Options

```text
-version            Show version
-help               Show help
-root string        Root path to scan (default: home)
-json               Output full JSON tree
-csv                Output flattened CSV
-no-interactive     Text summary mode
-whole-disk         Scan startup System/Data/VM volumes
-diagnose           Read-only diagnosis (whole disk unless a root is supplied)
-cross-filesystems  Include other mounted filesystems
-hidden=false       Exclude hidden files
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
d                   scrollable disk diagnosis
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

`MapMyStorage` ranks by physical size by default so the biggest entries have the most reported allocation (shared APFS clone blocks may overlap).

## Build From Source

```sh
git clone https://github.com/lautar0t/MapMyStorage.git
cd MapMyStorage
make build
./bin/mapmystorage -version
```

Useful development commands:

```sh
make test
make vet
make build-all
```

## Platform Notes

- macOS uses `st_blocks * 512`, File Provider path hints and `SF_DATALESS`.
- Linux uses allocated block metadata from `stat`.
- Windows uses Win32 APIs including `GetCompressedFileSizeW` and filesystem cluster size.
- Cloud provider metadata varies by provider, OS version, filesystem, and sync client configuration, so cloud status should be treated as a best-effort hint.

## License

MIT

## Diagnose a full Mac startup disk

```sh
# Interactive view: System + Data + VM; hidden files included
mapmystorage -whole-disk

# Text diagnosis, including APFS/Time Machine read-only checks
mapmystorage -diagnose

# Keep a shallower tree while still measuring deeper files
mapmystorage -diagnose -max-depth 5 -json > disk-diagnosis.json

# Focus on Dropbox or the often-large user Library
mapmystorage -diagnose -root "$HOME/Library/CloudStorage"
mapmystorage -root "$HOME/Library"
```

Press **d** in the TUI for the scrollable diagnosis. It shows filesystem capacity,
free/available space, scan coverage, cloud allocation, largest files and errors.
**v** shows the current folder's visual breakdown; **s** changes sort order;
**l** swaps allocated/logical columns (including their headings). The footer shows
the selected path and any error. Use **o** to reveal it in Finder.

Hidden files are included by default. `-hidden=false` excludes them in text/export
scans; in the TUI, **h** only changes visibility, so totals still include hidden
entries. `-max-depth` limits retained tree detail, not traversal: deep file sizes,
cloud statistics and the top 20 files/directories still contribute. Enter a summarized folder
to scan it as a new root. A full tree can use significant memory; depth 4–6 is
useful for a broad diagnosis.

By default, other mounted filesystems are skipped on Unix. On macOS,
`-whole-disk` includes the startup System, Data and VM volumes; it does not walk
external/network disks, Recovery, Preboot or backup mounts. APFS volume diagnostics
show the other volumes separately. Scan a mounted disk by passing its path, or
opt into `-cross-filesystems`. `-whole-disk` cannot be combined with a root path.
No files are deleted, evicted, hashed or read for their contents during scanning.
Dataless macOS directories are not enumerated, because enumeration itself can
cause materialization; their unknown descendants are explicitly reported.

### What the numbers mean

- **Allocated** is the filesystem's reported allocation (`st_blocks * 512` on
  macOS/Linux), including directory blocks. Zero blocks stays zero; no estimated
  inode overhead is added. Hard links and repeated directory references are
  counted once on Unix, with the original path recorded on subsequent references.
  Windows currently lacks this inode deduplication and Unix mount-boundary check.
- **Logical** is the sum of file lengths in the observed namespace. Multiple hard
  link names can contribute multiple logical lengths, but only one allocation.
- **Cloud status** uses provider-directory hints and OS evidence. On macOS,
  `SF_DATALESS` confirms online-only status. A tiny allocated/logical ratio alone
  does **not** prove a file is online-only or compressed. `unknown` stays unknown.
  Custom provider paths without recognizable names may not be attributed.
- **Cloud totals** cover identified files, not every byte used by the sync client.
  Dropbox/File Provider caches and databases under Library appear in folder totals.
- **Capacity minus free** can include other APFS volumes sharing a container.
  Its difference from the scanned allocation is not a measurement of “System
  Data”, purgeable space, or bytes that deletion will free. APFS clones share
  blocks; snapshots can retain deleted data; filesystem metadata and inaccessible
  paths also contribute. A negative difference is possible and is shown honestly.
- Output uses binary **KiB/MiB/GiB**. macOS Storage uses decimal **GB**; these units
  differ. Capacity is also shown in decimal GB for comparison.

### Incomplete scans and macOS privacy

Unreadable paths are marked **PARTIAL / ERROR**, never silently treated as empty.
Counts cover the whole scan; JSON preserves up to 100 issue paths plus the retained
entry errors. CSV includes `incomplete`, `scan_error`, `skipped`,
`counted_elsewhere`, and `summarized` columns. Root JSON includes a `report` with
coverage, volume metadata, cloud totals, largest files, config and OS diagnostics.
Depth/filtering, mounts and dataless directory omissions remain visible.

For protected user data, grant **Full Disk Access** in System Settings → Privacy &
Security to the terminal or host application that actually runs MapMyStorage,
then restart that application. `sudo` alone does not bypass macOS privacy. Some
system paths remain restricted. Run the diagnosis again after granting access.
If a snapshot query fails or times out, it is labeled **UNAVAILABLE**, not “none”.
Snapshot presence or “Purgeable: Yes” does not establish its reclaimable byte count.

Technical references: [Apple allocated blocks](https://developer.apple.com/documentation/system/stat/blocksallocated),
[Apple dataless files](https://developer.apple.com/documentation/technotes/tn3150-getting-ready-for-data-less-files),
[APFS space sharing, clones and sparse files](https://developer.apple.com/documentation/foundation/about-apple-file-system),
[Dropbox online-only files](https://help.dropbox.com/sync/make-files-online-only).
