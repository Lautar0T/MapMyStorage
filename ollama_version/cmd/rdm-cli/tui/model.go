package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lautaror/real-disk-map/core/disk"
	"github.com/lautaror/real-disk-map/core/export"
	"github.com/lautaror/real-disk-map/core/models"
)

// AppModel represents the state of the TUI application.
type AppModel struct {
	Config Config

	Root        *models.Entry
	CurrentDir  *models.Entry
	Breadcrumbs []string
	Children    []*models.Entry
	Cursor      int
	Offset      int

	Scanning     bool
	ScanError    error
	ScanProgress models.ScanProgress
	Width        int
	Height       int

	Spinner spinner.Model

	ShowHidden  bool
	SortBy      models.SortField
	SortOrder   models.SortOrder
	ShowLogical bool
	ShowSummary bool

	SearchMode    bool
	SearchQuery   string
	SearchResults []*models.Entry

	ExportMode   bool
	ExportFormat string

	Message     string
	MessageTime time.Time

	Keys KeyMap
}

// Config holds TUI-specific configuration.
type Config struct {
	RootPath        string
	FollowSymlinks  bool
	ExcludePatterns []string
	MaxDepth        int
	ShowHidden      bool
}

// KeyMap defines key bindings for the TUI.
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	Enter         key.Binding
	Back          key.Binding
	Quit          key.Binding
	Refresh       key.Binding
	Sort          key.Binding
	Search        key.Binding
	ToggleHidden  key.Binding
	ToggleSize    key.Binding
	ToggleSummary key.Binding
	Open          key.Binding
	Terminal      key.Binding
	Copy          key.Binding
	Export        key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace", "left"),
			key.WithHelp("←/back", "up dir"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		ToggleHidden: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "hidden"),
		),
		ToggleSize: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "real/logical"),
		),
		ToggleSummary: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "summary"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open/reveal"),
		),
		Terminal: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "terminal"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy path"),
		),
		Export: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export"),
		),
	}
}

// NewAppModel creates a new TUI model.
func NewAppModel(config Config) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return AppModel{
		Config:      config,
		Scanning:    true,
		Spinner:     s,
		ShowHidden:  config.ShowHidden,
		SortBy:      models.SortByPhysicalSize,
		SortOrder:   models.SortDescending,
		ShowLogical: false,
		ShowSummary: false,
		Breadcrumbs: make([]string, 0),
		Keys:        DefaultKeyMap(),
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.Spinner.Tick, StartScan(m.Config))
}

// ScanCompleteMsg is sent when scan completes.
type ScanCompleteMsg struct {
	Entry *models.Entry
	Err   error
}

type scanChannelMsg struct {
	Channel <-chan models.ScanEvent
	Err     error
}

type scanEventMsg struct {
	Channel <-chan models.ScanEvent
	Event   models.ScanEvent
	OK      bool
}

// StartScan starts scan operation and streams progress.
func StartScan(config Config) tea.Cmd {
	return func() tea.Msg {
		scanConfig := models.ScanConfig{
			RootPath:         config.RootPath,
			FollowSymlinks:   config.FollowSymlinks,
			ExcludePatterns:  config.ExcludePatterns,
			MaxDepth:         config.MaxDepth,
			ShowHidden:       true,
			IncludeCloudInfo: true,
		}

		scanner := disk.NewScanner(scanConfig)
		ch, err := scanner.ScanAsync(context.Background())
		return scanChannelMsg{Channel: ch, Err: err}
	}
}

func waitScanEvent(ch <-chan models.ScanEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		return scanEventMsg{Channel: ch, Event: event, OK: ok}
	}
}

// UpdateChildren refreshes current list after sort/filter changes.
func (m *AppModel) UpdateChildren() {
	if m.CurrentDir == nil {
		m.Children = nil
		return
	}

	opts := models.SortOptions{Field: m.SortBy, Order: m.SortOrder}
	m.Children = disk.GetEntryChildren(m.CurrentDir, opts, m.ShowHidden)

	if m.Cursor >= len(m.Children) {
		m.Cursor = len(m.Children) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
}

// EnterDir navigates into a directory.
func (m *AppModel) EnterDir(entry *models.Entry) {
	if entry == nil || entry.Type != models.EntryTypeDir {
		return
	}

	if m.CurrentDir != nil {
		m.Breadcrumbs = append(m.Breadcrumbs, m.CurrentDir.Path)
	}
	m.CurrentDir = entry
	m.Cursor = 0
	m.Offset = 0
	m.UpdateChildren()
}

// GoUp navigates to parent directory.
func (m *AppModel) GoUp() {
	if len(m.Breadcrumbs) == 0 {
		return
	}

	parentPath := m.Breadcrumbs[len(m.Breadcrumbs)-1]
	m.Breadcrumbs = m.Breadcrumbs[:len(m.Breadcrumbs)-1]

	if m.Root != nil {
		if found := models.FindEntry(m.Root, parentPath); found != nil {
			m.CurrentDir = found
		} else {
			m.CurrentDir = m.Root
		}
	}

	m.Cursor = 0
	m.Offset = 0
	m.UpdateChildren()
}

// CycleSort cycles through available sort options.
func (m *AppModel) CycleSort() {
	m.SortBy = (m.SortBy + 1) % 5
	m.UpdateChildren()
}

// ToggleSortOrder switches ascending/descending order.
func (m *AppModel) ToggleSortOrder() {
	if m.SortOrder == models.SortAscending {
		m.SortOrder = models.SortDescending
	} else {
		m.SortOrder = models.SortAscending
	}
	m.UpdateChildren()
}

// SetMessage sets temporary footer message.
func (m *AppModel) SetMessage(msg string) {
	m.Message = msg
	m.MessageTime = time.Now()
}

// ShouldClearMessage returns true when transient message expired.
func (m *AppModel) ShouldClearMessage() bool {
	return m.Message != "" && time.Since(m.MessageTime) > 3*time.Second
}

// GetSelected returns current selected entry.
func (m *AppModel) GetSelected() *models.Entry {
	if m.Cursor < 0 || m.Cursor >= len(m.Children) {
		return nil
	}
	return m.Children[m.Cursor]
}

// Search filters current directory children by case-insensitive substring.
func (m *AppModel) Search(query string) []*models.Entry {
	if query == "" {
		return nil
	}

	query = strings.ToLower(query)
	results := make([]*models.Entry, 0)
	for _, child := range m.Children {
		if strings.Contains(strings.ToLower(child.Name), query) {
			results = append(results, child)
		}
	}
	return results
}

// OpenInFileManager opens a path in Finder/Explorer/etc.
func OpenInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return exec.Command("explorer", path).Start()
	case "linux":
		return exec.Command("xdg-open", path).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

// OpenInTerminal opens terminal at path.
func OpenInTerminal(path string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal" to do script "cd %s"`, shellQuote(path))
		return exec.Command("osascript", "-e", script).Start()
	case "windows":
		if err := exec.Command("wt", "-d", path).Start(); err == nil {
			return nil
		}
		cmd := fmt.Sprintf(`Set-Location -LiteralPath '%s'`, strings.ReplaceAll(path, `'`, `''`))
		return exec.Command("powershell", "-NoExit", "-Command", cmd).Start()
	case "linux":
		return exec.Command("x-terminal-emulator", "--working-directory", path).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func shellQuote(path string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(path, "'", `'\\''`))
}

// RevealInFileManager reveals file in containing folder, or opens directory directly.
func RevealInFileManager(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return OpenInFileManager(path)
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,"+path).Start()
	default:
		return OpenInFileManager(filepath.Dir(path))
	}
}

// CopyToClipboard copies text to clipboard.
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// ExportToFile exports entry tree to JSON or CSV file in user home.
func ExportToFile(entry *models.Entry, format string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("real-disk-map_%s.%s", time.Now().Format("20060102_150405"), format)
	outPath := filepath.Join(home, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	switch format {
	case "json":
		err = export.ExportJSON(entry, f, export.ExportOptions{IncludeChildren: true})
	case "csv":
		err = export.ExportCSV(entry, f, export.ExportOptions{IncludeChildren: true})
	default:
		err = fmt.Errorf("unsupported export format: %s", format)
	}
	if err != nil {
		return "", err
	}

	return outPath, nil
}

// FormatBytes formats bytes to human-readable format.
func FormatBytes(b int64) string {
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

// EntryTypeIcon returns icon for entry kind.
func EntryTypeIcon(entry *models.Entry) string {
	switch entry.Type {
	case models.EntryTypeDir:
		return "📁"
	case models.EntryTypeSymlink:
		return "🔗"
	case models.EntryTypeFile:
		if entry.IsCloud {
			return "☁️"
		}
		return "📄"
	default:
		return "?"
	}
}

// EntrySizeColor returns a semantic color for physical size.
func EntrySizeColor(size int64, isPlaceholder bool) lipgloss.Color {
	if isPlaceholder || size == 0 {
		return lipgloss.Color("#6B7A8F")
	}
	switch {
	case size > 20*1024*1024*1024:
		return lipgloss.Color("#D7263D")
	case size > 5*1024*1024*1024:
		return lipgloss.Color("#F46036")
	case size > 512*1024*1024:
		return lipgloss.Color("#F6AE2D")
	default:
		return lipgloss.Color("#2E8B57")
	}
}

// EntryStatusIndicator returns status text for cloud/compression/sparse indicators.
func EntryStatusIndicator(entry *models.Entry) string {
	if entry.IsCloud {
		switch entry.CloudStatus {
		case models.CloudStatusOnlineOnly:
			return "online-only"
		case models.CloudStatusLocal:
			return "local"
		case models.CloudStatusSynced:
			return "synced"
		case models.CloudStatusDownloading:
			return "downloading"
		default:
			return "cloud"
		}
	}

	if entry.IsCompressed() {
		return "compressed"
	}
	if entry.IsSparse() {
		return "sparse"
	}
	return "local"
}
