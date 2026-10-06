package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/lautar0t/MapMyStorage/cmd/mapmystorage/tui"
	"github.com/lautar0t/MapMyStorage/core/disk"
	"github.com/lautar0t/MapMyStorage/core/export"
	"github.com/lautar0t/MapMyStorage/core/models"
	"github.com/lautar0t/MapMyStorage/core/report"
)

var version = "dev"

func main() {
	var (
		wholeDisk      = flag.Bool("whole-disk", false, "Scan startup disk including hidden data (macOS: System, Data, VM)")
		diagnose       = flag.Bool("diagnose", false, "Read-only disk diagnosis (whole disk unless a root is supplied)")
		crossFS        = flag.Bool("cross-filesystems", false, "Include other mounted filesystems; may include network/external disks")
		showVersion    = flag.Bool("version", false, "Show version")
		showHelp       = flag.Bool("help", false, "Show help")
		rootPath       = flag.String("root", "", "Root path to scan (default: home directory)")
		jsonOutput     = flag.Bool("json", false, "Output as JSON")
		csvOutput      = flag.Bool("csv", false, "Output as CSV")
		noInteractive  = flag.Bool("no-interactive", false, "Run without interactive TUI")
		showHidden     = flag.Bool("hidden", true, "Include hidden files (default true)")
		followSymlinks = flag.Bool("follow-symlinks", false, "Follow symbolic links")
		maxDepth       = flag.Int("max-depth", 0, "Maximum scan depth (0 = unlimited)")
		exclude        = flag.String("exclude", "", "Comma-separated exclude patterns")
	)

	flag.BoolVar(showHelp, "h", false, "Show help")
	flag.Usage = func() { printUsage(os.Stderr) }
	flag.Parse()

	if *showVersion {
		fmt.Printf("MapMyStorage version %s\n", version)
		return
	}
	if *showHelp {
		printUsage(os.Stdout)
		return
	}

	if err := validateFlags(*jsonOutput, *csvOutput, *maxDepth); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid options: %v\n\n", err)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	if err := validatePaths(*rootPath, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid options: %v\n", err)
		os.Exit(2)
	}
	if *wholeDisk && (*rootPath != "" || len(flag.Args()) > 0) {
		fmt.Fprintln(os.Stderr, "-whole-disk cannot be combined with an explicit path")
		os.Exit(2)
	}
	if *diagnose && *rootPath == "" && len(flag.Args()) == 0 {
		*wholeDisk = true
	}
	scanRoot := resolveRoot(*rootPath, flag.Args())
	if *wholeDisk {
		scanRoot = string(filepath.Separator)
		if runtime.GOOS == "windows" {
			scanRoot = filepath.VolumeName(os.Getenv("SystemRoot")) + string(filepath.Separator)
		}
		*showHidden = true
	}
	absRoot, err := filepath.Abs(scanRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}
	if err := validateRoot(absRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid root path %q: %v\n", absRoot, err)
		os.Exit(1)
	}

	excludePatterns := parseExcludePatterns(*exclude)
	scanConfig := models.ScanConfig{
		RootPath:         absRoot,
		FollowSymlinks:   *followSymlinks,
		ExcludePatterns:  excludePatterns,
		MaxDepth:         *maxDepth,
		ShowHidden:       *showHidden,
		IncludeCloudInfo: true,
		CrossFilesystems: *crossFS, WholeDisk: *wholeDisk, Diagnostics: *diagnose,
	}

	if !*jsonOutput && !*csvOutput && !*noInteractive && !*diagnose && interactiveTerminal() {
		appCfg := tui.Config{
			RootPath:         absRoot,
			CrossFilesystems: *crossFS, WholeDisk: *wholeDisk,
			FollowSymlinks:  *followSymlinks,
			ExcludePatterns: excludePatterns,
			MaxDepth:        *maxDepth,
			ShowHidden:      *showHidden,
		}
		model := tui.NewAppModel(appCfg)
		defer model.ScanCancel()
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	scanner := disk.NewScanner(scanConfig)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	entry, err := scanWithProgress(ctx, scanner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *jsonOutput:
		if err := export.ExportJSON(entry, os.Stdout, export.ExportOptions{IncludeChildren: true, MaxDepth: *maxDepth}); err != nil {
			fmt.Fprintf(os.Stderr, "JSON export failed: %v\n", err)
			os.Exit(1)
		}
	case *csvOutput:
		if err := export.ExportCSV(entry, os.Stdout, export.ExportOptions{IncludeChildren: true, MaxDepth: *maxDepth}); err != nil {
			fmt.Fprintf(os.Stderr, "CSV export failed: %v\n", err)
			os.Exit(1)
		}
	default:
		printTextSummary(entry)
	}
}

func validateFlags(jsonOutput, csvOutput bool, maxDepth int) error {
	if jsonOutput && csvOutput {
		return fmt.Errorf("-json and -csv cannot be used together")
	}
	if maxDepth < 0 {
		return fmt.Errorf("-max-depth must be greater than or equal to 0")
	}
	return nil
}

func validateRoot(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist")
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied")
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("symlink target is not accessible: %w", err)
		}
		if target.IsDir() || target.Mode().IsRegular() {
			return nil
		}
		return fmt.Errorf("symlink target is not a regular file or directory")
	}
	if info.IsDir() || info.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("path is not a regular file or directory")
}

func resolveRoot(rootFlag string, args []string) string {
	if rootFlag != "" {
		return rootFlag
	}
	if len(args) > 0 {
		return args[0]
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func printTextSummary(root *models.Entry) { report.Write(os.Stdout, root) }

func printUsage(w io.Writer) {
	fmt.Fprint(w, `MapMyStorage — allocated disk usage, logical sizes and cloud diagnostics

Usage:
  mapmystorage [options] [path]

Examples:
  mapmystorage --whole-disk                    Explore the startup disk
  mapmystorage --diagnose --max-depth 5         Full-disk text diagnosis
  mapmystorage --root ~/Library                Explore application data
  mapmystorage --json --root ~/Downloads > disk.json
  mapmystorage --csv --root ~/Downloads > disk.csv
  mapmystorage --exclude ".git,node_modules" --root ~/Projects

Options (both -option and --option are accepted):
  --help, -h          Show this help and exit
  --version          Show the build version
  --root PATH        Root to scan (default: home; or use one positional path)
  --whole-disk       Startup disk: macOS System, Data and VM; includes hidden files
  --diagnose         Text diagnosis and read-only OS checks; defaults to whole disk
                     unless --root or a positional path is supplied
  --json             JSON tree, totals, coverage and diagnostics (when requested)
  --csv              CSV rows with allocated/logical sizes and coverage flags
  --no-interactive   Text report; also selected automatically when input/output
                     is not an interactive terminal
  --hidden=false     Exclude hidden entries in text/exports; hide them in the TUI
                     (default: included; --whole-disk always includes them)
  --follow-symlinks  Follow descendant symbolic links (default: false)
  --max-depth N      Retained tree depth (0 = unlimited); still measures deeper files
  --exclude GLOBS    Comma-separated names/globs to omit from the scan
  --cross-filesystems
                     Traverse other mounts, including external/network disks
                     (default on Unix: stay on the root filesystem, except the
                     startup volumes included by --whole-disk)

Put options before the positional path. Use -- before a path beginning with '-'.
Do not combine --root with a positional path, --whole-disk with a path, or --json
with --csv. --diagnose can be combined with --json or --csv. Full trees may use
significant memory; --max-depth 5 keeps large diagnoses compact.

Sizes and coverage:
  Allocated = filesystem blocks, not guaranteed space freed by deleting files.
  Logical = file lengths; sparse, compressed and cloud files can appear much larger.
  GiB is binary; macOS Storage uses decimal GB. APFS clones, snapshots, metadata
  and inaccessible paths mean totals need not equal the container's used space.
  Errors/omissions remain visible as PARTIAL; unknown usage is not zero.
  The scanner does not read file contents or delete files. macOS dataless
  directories are not enumerated to avoid triggering materialization.
  On macOS, grant Full Disk Access to the terminal/host app and restart it for
  protected data. sudo alone does not bypass privacy restrictions.
  Whole-disk mode omits Recovery, Preboot and external/backup mounts; macOS
  diagnosis lists APFS volumes and snapshots separately. Windows currently lacks
  Unix inode deduplication and mount-boundary checks.

TUI shortcuts:
  arrows / j,k       move
  Enter              enter directory / reveal file
  Backspace / Left   go to parent
  r                  rescan current directory as a new root
  s                  cycle sort
  /                  search current directory
  h                  toggle hidden entries (totals still include them)
  l                  swap allocated/logical columns
  v                  visual summary panel
  d                  diagnosis (arrows/PgUp/PgDn scroll; Esc closes)
  o                  open/reveal in Finder/Explorer
  t                  open in terminal
  y                  copy path
  e                  export JSON/CSV to your home directory
  q / Ctrl+C         quit / cancel scan

Exit codes: 0 = completed (possibly partial), 1 = scan/output/runtime failure,
            2 = invalid arguments. Check report coverage before trusting totals.
`)
}

func parseExcludePatterns(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validatePaths(root string, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("accepts one path; place all options before the path")
	}
	if root != "" && len(args) > 0 {
		return fmt.Errorf("--root cannot be combined with a positional path")
	}
	return nil
}

func interactiveTerminal() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

// Keep long text/JSON scans observable without contaminating stdout exports.
func scanWithProgress(ctx context.Context, scanner *disk.Scanner) (*models.Entry, error) {
	ch, err := scanner.ScanAsync(ctx)
	if err != nil {
		return nil, err
	}
	info, _ := os.Stderr.Stat()
	terminal := info != nil && term.IsTerminal(os.Stderr.Fd())
	last := time.Time{}
	for event := range ch {
		switch event.Type {
		case models.ScanEventProgress:
			if terminal && event.Progress != nil && time.Since(last) >= time.Second {
				last = time.Now()
				p := event.Progress
				fmt.Fprintf(os.Stderr, "Scanned %d files, %d dirs, %s allocated, %d errors\n", p.FilesScanned, p.DirsScanned, report.Bytes(p.BytesProcessed), p.ErrorsEncountered)
			}
		case models.ScanEventCompleted:
			return event.Entry, nil
		case models.ScanEventError:
			return nil, event.Error
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("scan ended without a result")
}
