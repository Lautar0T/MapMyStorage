package tui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lautaror/real-disk-map/core/models"
)

// Update handles messages and updates the model.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, m.Keys.Quit) {
			return m, tea.Quit
		}

		if m.SearchMode {
			return m.handleSearchKey(msg)
		}

		if m.ExportMode {
			return m.handleExportKey(msg)
		}

		if m.Scanning {
			return m, nil
		}

		return m.handleNormalKey(msg)

	case scanChannelMsg:
		if msg.Err != nil {
			m.Scanning = false
			m.ScanError = msg.Err
			return m, nil
		}
		m.Scanning = true
		m.ScanError = nil
		m.ScanProgress = models.ScanProgress{}
		return m, waitScanEvent(msg.Channel)

	case scanEventMsg:
		if !msg.OK {
			m.Scanning = false
			return m, nil
		}

		switch msg.Event.Type {
		case models.ScanEventStarted:
			m.Scanning = true
			m.ScanError = nil
		case models.ScanEventProgress:
			if msg.Event.Progress != nil {
				m.ScanProgress = *msg.Event.Progress
			}
		case models.ScanEventError:
			m.Scanning = false
			m.ScanError = msg.Event.Error
			return m, nil
		case models.ScanEventCompleted:
			m.Scanning = false
			m.Root = msg.Event.Entry
			m.CurrentDir = msg.Event.Entry
			m.Breadcrumbs = nil
			m.Cursor = 0
			m.Offset = 0
			m.UpdateChildren()
			return m, nil
		}

		return m, waitScanEvent(msg.Channel)

	case ScanCompleteMsg:
		m.Scanning = false
		if msg.Err != nil {
			m.ScanError = msg.Err
		} else {
			m.Root = msg.Entry
			m.CurrentDir = msg.Entry
			m.Breadcrumbs = []string{}
			m.UpdateChildren()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	default:
		return m, nil
	}
}

func (m AppModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.Keys.Up):
		if m.Cursor > 0 {
			m.Cursor--
		}
		m.adjustOffset()

	case key.Matches(msg, m.Keys.Down):
		if m.Cursor < len(m.Children)-1 {
			m.Cursor++
		}
		m.adjustOffset()

	case key.Matches(msg, m.Keys.Enter):
		if selected := m.GetSelected(); selected != nil {
			if selected.Type == models.EntryTypeDir {
				m.EnterDir(selected)
			} else {
				if err := RevealInFileManager(selected.Path); err != nil {
					m.SetMessage(fmt.Sprintf("Error opening: %v", err))
				}
			}
		}

	case key.Matches(msg, m.Keys.Back):
		m.GoUp()

	case key.Matches(msg, m.Keys.Refresh):
		m.Scanning = true
		m.ScanError = nil
		m.ScanProgress = models.ScanProgress{}
		m.Cursor = 0
		m.Offset = 0
		cfg := m.Config
		if m.CurrentDir != nil {
			cfg.RootPath = m.CurrentDir.Path
		}
		return m, tea.Batch(m.Spinner.Tick, StartScan(cfg))

	case key.Matches(msg, m.Keys.Sort):
		m.CycleSort()

	case key.Matches(msg, m.Keys.Search):
		m.SearchMode = true
		m.SearchQuery = ""
		m.SearchResults = nil

	case key.Matches(msg, m.Keys.ToggleHidden):
		m.ShowHidden = !m.ShowHidden
		m.UpdateChildren()

	case key.Matches(msg, m.Keys.ToggleSize):
		m.ShowLogical = !m.ShowLogical

	case key.Matches(msg, m.Keys.ToggleSummary):
		m.ShowSummary = !m.ShowSummary

	case key.Matches(msg, m.Keys.Open):
		if selected := m.GetSelected(); selected != nil {
			if err := RevealInFileManager(selected.Path); err != nil {
				m.SetMessage(fmt.Sprintf("Error opening: %v", err))
			} else {
				m.SetMessage("Opened in file manager")
			}
		}

	case key.Matches(msg, m.Keys.Terminal):
		if selected := m.GetSelected(); selected != nil {
			target := selected.Path
			if selected.Type != models.EntryTypeDir {
				target = filepath.Dir(selected.Path)
			}
			if err := OpenInTerminal(target); err != nil {
				m.SetMessage(fmt.Sprintf("Error opening terminal: %v", err))
			} else {
				m.SetMessage("Opened in terminal")
			}
		}

	case key.Matches(msg, m.Keys.Copy):
		if selected := m.GetSelected(); selected != nil {
			if err := CopyToClipboard(selected.Path); err != nil {
				m.SetMessage(fmt.Sprintf("Error copying: %v", err))
			} else {
				m.SetMessage("Path copied")
			}
		}

	case key.Matches(msg, m.Keys.Export):
		m.ExportMode = true
		m.ExportFormat = "json"
	}

	return m, nil
}

func (m AppModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.SearchMode = false
		m.SearchQuery = ""
		m.SearchResults = nil

	case tea.KeyEnter:
		m.SearchMode = false
		if len(m.SearchResults) > 0 {
			for i, child := range m.Children {
				if child == m.SearchResults[0] {
					m.Cursor = i
					m.adjustOffset()
					break
				}
			}
		}

	case tea.KeyBackspace:
		if len(m.SearchQuery) > 0 {
			m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
			m.SearchResults = m.Search(m.SearchQuery)
		}

	case tea.KeyRunes:
		m.SearchQuery += string(msg.Runes)
		m.SearchResults = m.Search(m.SearchQuery)
	}

	return m, nil
}

func (m AppModel) handleExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ExportMode = false
	case "j":
		m.ExportFormat = "json"
	case "c":
		m.ExportFormat = "csv"
	case "enter":
		m.ExportMode = false
		if m.Root != nil {
			path, err := exportToFile(m.Root, m.ExportFormat)
			if err != nil {
				m.SetMessage(fmt.Sprintf("Export failed: %v", err))
			} else {
				m.SetMessage(fmt.Sprintf("Exported to %s", path))
			}
		}
	}

	return m, nil
}

func (m *AppModel) adjustOffset() {
	listHeight := m.Height - 12
	if m.ShowSummary {
		listHeight -= 8
	}
	if listHeight < 5 {
		listHeight = 5
	}

	if m.Cursor < m.Offset {
		m.Offset = m.Cursor
	} else if m.Cursor >= m.Offset+listHeight {
		m.Offset = m.Cursor - listHeight + 1
	}

	if m.Offset < 0 {
		m.Offset = 0
	}

	if len(m.Children) > 0 && m.Offset > len(m.Children)-listHeight {
		m.Offset = len(m.Children) - listHeight
		if m.Offset < 0 {
			m.Offset = 0
		}
	}
}

func exportToFile(entry *models.Entry, format string) (string, error) {
	return ExportToFile(entry, format)
}

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#384B70")).
			PaddingLeft(1).
			PaddingRight(1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			PaddingLeft(1)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#25364F")).
			Foreground(lipgloss.Color("#FAFAFA"))

	normalStyle = lipgloss.NewStyle()

	dirStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4DA3FF"))

	symlinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ADD8"))

	cloudStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8AA5C2"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00C853")).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6AB7FF"))
)
