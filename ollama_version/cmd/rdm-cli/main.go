package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lautaror/real-disk-map/cmd/rdm-cli/tui"
	"github.com/lautaror/real-disk-map/core/disk"
	"github.com/lautaror/real-disk-map/core/export"
	"github.com/lautaror/real-disk-map/core/models"
)

const version = "0.2.0"

func main() {
	var (
		showVersion    = flag.Bool("version", false, "Show version")
		showHelp       = flag.Bool("help", false, "Show help")
		rootPath       = flag.String("root", "", "Root path to scan (default: home directory)")
		jsonOutput     = flag.Bool("json", false, "Output as JSON")
		csvOutput      = flag.Bool("csv", false, "Output as CSV")
		noInteractive  = flag.Bool("no-interactive", false, "Run without interactive TUI")
		showHidden     = flag.Bool("hidden", false, "Show hidden files")
		followSymlinks = flag.Bool("follow-symlinks", false, "Follow symbolic links")
		maxDepth       = flag.Int("max-depth", 0, "Maximum scan depth (0 = unlimited)")
		exclude        = flag.String("exclude", "", "Comma-separated exclude patterns")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *showVersion {
		fmt.Printf("real-disk-map version %s\n", version)
		return
	}
	if *showHelp {
		printUsage()
		return
	}

	scanRoot := resolveRoot(*rootPath, flag.Args())
	absRoot, err := filepath.Abs(scanRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
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
	}

	if !*jsonOutput && !*csvOutput && !*noInteractive {
		appCfg := tui.Config{
			RootPath:        absRoot,
			FollowSymlinks:  *followSymlinks,
			ExcludePatterns: excludePatterns,
			MaxDepth:        *maxDepth,
			ShowHidden:      *showHidden,
		}
		model := tui.NewAppModel(appCfg)
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	scanner := disk.NewScanner(scanConfig)
	entry, err := scanner.Scan()
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

func printTextSummary(root *models.Entry) {
	fmt.Printf("Root: %s\n", root.Path)
	fmt.Printf("Total real:    %s\n", formatBytes(root.TotalPhysicalSize()))
	fmt.Printf("Total logical: %s\n", formatBytes(root.TotalLogicalSize()))

	if root.TotalLogicalSize() > 0 {
		delta := root.TotalLogicalSize() - root.TotalPhysicalSize()
		fmt.Printf("Difference:    %s\n", formatBytes(delta))
	}

	all := make([]*models.Entry, 0, 1024)
	_ = models.WalkEntry(root, func(e *models.Entry) error {
		if e.Path != root.Path {
			all = append(all, e)
		}
		return nil
	})

	sort.Slice(all, func(i, j int) bool {
		return all[i].PhysicalSize > all[j].PhysicalSize
	})

	fmt.Println("\nTop 20 by real disk usage:")
	for i, e := range all {
		if i >= 20 {
			break
		}
		kind := "F"
		if e.Type == models.EntryTypeDir {
			kind = "D"
		}
		fmt.Printf("%2d. [%s] %10s  %s\n", i+1, kind, formatBytes(e.PhysicalSize), e.Path)
	}

	fmt.Println("\nLevel summary (children of current root):")
	for _, child := range root.Children {
		fmt.Printf("%-40s %10s\n", truncate(child.Name, 40), formatBytes(child.TotalPhysicalSize()))
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `real-disk-map (rdm-cli) - real allocated disk usage analyzer

Usage:
  rdm-cli [options] [path]

Options:
  -version            Show version
  -help               Show this help
  -root string        Root path to scan (default: home)
  -json               Output full JSON tree
  -csv                Output flattened CSV
  -no-interactive     Text summary mode
  -hidden             Show hidden files
  -follow-symlinks    Follow symbolic links (default: false)
  -max-depth int      Max depth (0 unlimited)
  -exclude string     Comma-separated exclude patterns

TUI shortcuts:
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
