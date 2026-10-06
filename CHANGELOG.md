# Changelog

## Unreleased

- Added startup-disk scanning and read-only diagnosis with APFS/Time Machine checks.
- Resolve arbitrary scan roots to their containing volume before calling diskutil.
- Report allocated and logical sizes, cloud totals, largest files and scan coverage.
- Include hidden files by default; preserve permission errors in TUI, JSON and CSV.
- Keep zero allocation accurate; deduplicate Unix hard links and macOS firmlinks.
- Detect macOS dataless files and skip dataless directory enumeration.
- Avoid network/external mounts before stat on macOS; allow explicit opt-in.
- Bound retained tree depth without losing totals, cloud accounting or large files.
- Add cancellation, terminal progress, diagnosis navigation and correct size headings.
- Expand --help/-h with examples, limits and exit codes; reject conflicting roots and extra arguments.
- Select text mode automatically outside interactive terminals.
- Install source builds to ~/.local/bin by default and fail cross-builds on errors.
- Cover help, validation, redirected text and JSON/CSV with end-to-end CLI tests.

## v0.2.0

- Initial public CLI release.
- Added interactive TUI, JSON export, and CSV export.
- Added physical disk usage scanning for macOS, Linux, and Windows.
- Added best-effort cloud placeholder detection.
- Added release packaging for GitHub Releases.
