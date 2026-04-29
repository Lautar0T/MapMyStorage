//go:build ignore
// +build ignore

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lautaror/real-disk-map/core/disk"
	"github.com/lautaror/real-disk-map/core/export"
	"github.com/lautaror/real-disk-map/core/models"
)

// Config holds the CLI configuration.
type Config struct {
	RootPath        string
	JsonOutput      bool
	CsvOutput       bool
	NoInteractive   bool
	ShowHidden      bool
	FollowSymlinks  bool
	MaxDepth        int
	ExcludePatterns []string
}

// Execute runs the appropriate command based on configuration.
func Execute(config Config) error {
	// If JSON or CSV output is requested, run in batch mode
	if config.JsonOutput || config.CsvOutput {
		return runBatchMode(config)
	}

	// If no-interactive flag is set, run in batch mode with text output
	if config.NoInteractive {
		return runTextMode(config)
	}

	// Otherwise run interactive TUI
	return runInteractiveMode(config)
}

// runBatchMode runs a batch scan and outputs to JSON or CSV.
func runBatchMode(config Config) error {
	fmt.Fprintf(os.Stderr, "Scanning %s...\n", config.RootPath)

	scanConfig := models.ScanConfig{
		RootPath:         config.RootPath,
		FollowSymlinks:   config.FollowSymlinks,
		ExcludePatterns:  config.ExcludePatterns,
		MaxDepth:         config.MaxDepth,
		ShowHidden:       config.ShowHidden,
		IncludeCloudInfo: true,
	}

	scanner := disk.NewScanner(scanConfig)
	start := time.Now()

	entry, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	duration := time.Since(start)
	fmt.Fprintf(os.Stderr, "Scan completed in %v\n", duration)

	// Output results
	if config.JsonOutput {
		return export.ExportJSON(entry, os.Stdout, export.ExportOptions{
			IncludeChildren: true,
		})
	}

	if config.CsvOutput {
		return export.ExportCSV(entry, os.Stdout, export.ExportOptions{
			IncludeChildren: true,
			MaxDepth:        config.MaxDepth,
		})
	}

	return nil
}

// runTextMode runs a batch scan and outputs text summary.
func runTextMode(config Config) error {
	fmt.Fprintf(os.Stderr, "Scanning %s...\n", config.RootPath)

	scanConfig := models.ScanConfig{
		RootPath:         config.RootPath,
		FollowSymlinks:   config.FollowSymlinks,
		ExcludePatterns:  config.ExcludePatterns,
		MaxDepth:         config.MaxDepth,
		ShowHidden:       config.ShowHidden,
		IncludeCloudInfo: true,
	}

	scanner := disk.NewScanner(scanConfig)
	start := time.Now()

	entry, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	duration := time.Since(start)

	// Print summary
	fmt.Printf("\n%s\n", lipgloss.NewStyle().Bold(true).Render("Scan Results"))
	fmt.Printf("Root: %s\n", entry.Path)
	fmt.Printf("Duration: %v\n\n", duration)

	fmt.Printf("Total Physical Size: %s\n", formatBytes(entry.TotalPhysicalSize()))
	fmt.Printf("Total Logical Size:  %s\n", formatBytes(entry.TotalLogicalSize()))

	if entry.TotalLogicalSize() > entry.TotalPhysicalSize() {
		saved := entry.TotalLogicalSize() - entry.TotalPhysicalSize()
		percent := float64(saved) * 100 / float64(entry.TotalLogicalSize())
		fmt.Printf("Space Saved: %s (%.1f%%)\n", formatBytes(saved), percent)
	}

	fmt.Println("\nTop 20 by Physical Size:")
	printTopEntries(entry, 20)

	return nil
}

// printTopEntries prints the top N entries by physical size.
func printTopEntries(root *models.Entry, n int) {
	// Flatten and collect all entries
	var entries []*models.Entry
	models.WalkEntry(root, func(e *models.Entry) error {
		entries = append(entries, e)
		return nil
	})

	// Sort by physical size
	models.SortDescendingEntries(entries, models.SortByPhysicalSize)

	// Print top N
	for i, e := range entries {
		if i >= n {
			break
		}

		indicator := ""
		if e.Type == models.EntryTypeDir {
			indicator = "/"
		}

		cloud := ""
		if e.IsCloud {
			cloud = " [☁️ " + string(e.CloudProvider) + "]"
		}

		fmt.Printf("%10s  %s%s%s\n", formatBytes(e.PhysicalSize), e.Path, indicator, cloud)
	}
}

// runInteractiveMode runs the Bubble Tea TUI.
func runInteractiveMode(config Config) error {
	m := initialModel(config)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// ParseExcludePatterns parses a comma-separated string of exclude patterns.
func ParseExcludePatterns(s string) []string {
	if s == "" {
		return nil
	}

	patterns := strings.Split(s, ",")
	for i := range patterns {
		patterns[i] = strings.TrimSpace(patterns[i])
	}
	return patterns
}

// formatBytes formats bytes to human-readable string.
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

// Model represents the state of the TUI.
type model struct {
	config      Config
	scanner     *disk.Scanner
	root        *models.Entry
	currentDir  *models.Entry
	breadcrumbs []*models.Entry
	children    []*models.Entry
	cursor      int
	viewport    int
	totalHeight int

	// Scanning state
	scanning     bool
	scanProgress models.ScanProgress
	spinner      spinner.Model
	progressBar  progress.Model
	err          error

	// View state
	showHidden      bool
	sortBy          models.SortField
	sortOrder       models.SortOrder
	showLogicalSize bool
	searchQuery     string
	searching       bool

	// Export
	exporting    bool
	exportFormat string

	// Messages
	message      string
	messageTimer time.Time
}

// initialModel creates the initial model for the TUI.
func initialModel(config Config) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	showHidden := config.ShowHidden
	sortBy := models.SortByPhysicalSize
	sortOrder := models.SortDescending

	m := model{
		config:      config,
		spinner:     s,
		progressBar: prog,
		showHidden:  showHidden,
		sortBy:      sortBy,
		sortOrder:   sortOrder,
		breadcrumbs: make([]*models.Entry, 0),
	}

	return m
}

// Init initializes the TUI.
func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		startScan(m.config),
	)
}

// startScan starts the scan operation.
func startScan(config Config) tea.Cmd {
	return func() tea.Msg {
		scanConfig := models.ScanConfig{
			RootPath:         config.RootPath,
			FollowSymlinks:   config.FollowSymlinks,
			ExcludePatterns:  config.ExcludePatterns,
			MaxDepth:         config.MaxDepth,
			ShowHidden:       config.ShowHidden,
			IncludeCloudInfo: true,
		}

		scanner := disk.NewScanner(scanConfig)
		entry, err := scanner.Scan()

		return scanCompleteMsg{
			entry: entry,
			err:   err,
		}
	}
}

// scanCompleteMsg is sent when scanning completes.
type scanCompleteMsg struct {
	entry *models.Entry
	err   error
}

// Update updates the model based on messages.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.totalHeight = msg.Height
		return m, nil

	case scanCompleteMsg:
		m.scanning = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.root = msg.entry
			m.currentDir = msg.entry
			m.breadcrumbs = []*models.Entry{msg.entry}
			m.updateChildren()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progressBar, cmd = m.progressBar.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKey handles keyboard input.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.handleSearchKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.updateViewport()

	case "down", "j":
		if m.cursor < len(m.children)-1 {
			m.cursor++
		}
		m.updateViewport()

	case "enter":
		if len(m.children) > 0 && m.cursor < len(m.children) {
			selected := m.children[m.cursor]
			if selected.Type == models.EntryTypeDir {
				m.enterDir(selected)
			}
		}

	case "backspace", "left", "h":
		if len(m.breadcrumbs) > 1 {
			m.goUp()
		}

	case "r":
		return m, startScan(m.config)

	case "s":
		m.cycleSort()

	case "/":
		m.searching = true
		m.searchQuery = ""

	case "l":
		m.showLogicalSize = !m.showLogicalSize

	case "t":
		if len(m.children) > 0 && m.cursor < len(m.children) {
			openInTerminal(m.children[m.cursor].Path)
		}

	case "o":
		if len(m.children) > 0 && m.cursor < len(m.children) {
			openInFileManager(m.children[m.cursor].Path)
		}

	case "y":
		if len(m.children) > 0 && m.cursor < len(m.children) {
			copyToClipboard(m.children[m.cursor].Path)
			m.message = "Path copied to clipboard"
			m.messageTimer = time.Now()
		}

	case "e":
		// Export current view
		m.exporting = true
		m.exportFormat = "json"
	}

	return m, nil
}

// handleSearchKey handles keys during search mode.
func (m model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchQuery = ""
	case "enter":
		m.searching = false
		// Find and jump to matching entry
		m.jumpToSearch()
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.searchQuery += msg.String()
		}
	}
	return m, nil
}

// View renders the TUI.
func (m model) View() string {
	if m.scanning {
		return m.renderScanningView()
	}

	if m.err != nil {
		return m.renderErrorView()
	}

	return m.renderMainView()
}

// renderScanningView renders the scanning progress view.
func (m model) View() string {
	return m.spinner.View() + " Scanning..."
}

// Helper functions (placeholders - will be implemented)
func (m *model) updateChildren()              {}
func (m *model) updateViewport()              {}
func (m *model) enterDir(entry *models.Entry) {}
func (m *model) goUp()                        {}
func (m *model) cycleSort()                   {}
func (m *model) jumpToSearch()                {}
func (m model) renderScanningView() string    { return "" }
func (m model) renderErrorView() string       { return "" }
func (m model) renderMainView() string        { return "" }
func openInTerminal(path string)              {}
func openInFileManager(path string)           {}
func copyToClipboard(text string)             {}
