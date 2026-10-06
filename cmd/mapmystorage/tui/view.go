package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lautar0t/MapMyStorage/core/models"
	"github.com/lautar0t/MapMyStorage/core/report"
)

// View renders the TUI.
func (m AppModel) View() string {
	if m.ShouldClearMessage() {
		m.Message = ""
	}

	if m.Scanning {
		return m.renderScanningView()
	}
	if m.ScanError != nil {
		return m.renderErrorView()
	}
	if m.Diagnosis {
		return m.renderDiagnosis()
	}
	if m.ExportMode {
		return m.renderExportView()
	}
	return m.renderMainView()
}

func (m AppModel) renderScanningView() string {
	progress := lipgloss.NewStyle().Foreground(lipgloss.Color("#90A4AE")).Render(
		fmt.Sprintf(
			"Files: %d  Dirs: %d  Real: %s  Errors: %d",
			m.ScanProgress.FilesScanned,
			m.ScanProgress.DirsScanned,
			FormatBytes(m.ScanProgress.BytesProcessed),
			m.ScanProgress.ErrorsEncountered,
		),
	)
	current := ""
	if m.ScanProgress.CurrentPath != "" {
		current = truncate(m.ScanProgress.CurrentPath, maxInt(40, m.Width-8))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		spinnerStyle.Render(m.Spinner.View()+" Scanning filesystem..."),
		"",
		progress,
		current,
	)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
}

func (m AppModel) renderErrorView() string {
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D7263D")).Bold(true)
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		errorStyle.Render("Scan Error"),
		"",
		fmt.Sprintf("%v", m.ScanError),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#777")).Render("Press q to quit"),
	)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
}

func (m AppModel) renderMainView() string {
	if m.CurrentDir == nil {
		return ""
	}

	parts := []string{m.renderHeader(), m.renderCoverage()}
	if m.SearchMode {
		parts = append(parts, m.renderSearchBar())
	}
	parts = append(parts, m.renderFileList())

	if m.ShowSummary {
		parts = append(parts, m.renderSummaryPanel())
	}
	if m.Message != "" {
		parts = append(parts, messageStyle.Render(m.Message))
	}
	parts = append(parts, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m AppModel) renderExportView() string {
	options := []string{"Export as JSON (j)", "Export as CSV (c)"}
	if m.ExportFormat == "json" {
		options[0] = "▸ " + options[0]
		options[1] = "  " + options[1]
	} else {
		options[0] = "  " + options[0]
		options[1] = "▸ " + options[1]
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("Export Results"),
		"",
		strings.Join(options, "\n"),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).Render("Enter: confirm, Esc: cancel"),
	)

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(content)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m AppModel) renderHeader() string {
	crumbs := make([]string, 0, len(m.Breadcrumbs)+2)
	if m.Root != nil {
		rootName := m.Root.Name
		if rootName == "" {
			rootName = m.Root.Path
		}
		crumbs = append(crumbs, rootName)
	}
	for _, b := range m.Breadcrumbs {
		crumbs = append(crumbs, getName(b))
	}
	if m.CurrentDir != nil && m.CurrentDir != m.Root {
		crumbs = append(crumbs, m.CurrentDir.Name)
	}

	breadcrumb := strings.Join(crumbs, " / ")
	stats := ""
	if m.CurrentDir != nil {
		stats = fmt.Sprintf("Allocated: %s | Logical: %s", FormatBytes(m.CurrentDir.TotalPhysicalSize()), FormatBytes(m.CurrentDir.TotalLogicalSize()))
	}

	line := fmt.Sprintf("%s  %s", breadcrumb, stats)
	if m.Width > 0 {
		line = truncate(line, m.Width-2)
	}
	return headerStyle.Width(maxInt(0, m.Width)).Render(line)
}

func (m AppModel) renderSearchBar() string {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#2E3C53")).
		Foreground(lipgloss.Color("#FAFAFA")).
		PaddingLeft(1).
		Width(maxInt(0, m.Width)).
		Render("Search: " + m.SearchQuery + "_")
}

func (m AppModel) renderFileList() string {
	listHeight := m.computeListHeight()
	lines := make([]string, 0, listHeight+2)

	primary, secondary := "Allocated", "Logical"
	if m.ShowLogical {
		primary, secondary = secondary, primary
	}
	header := m.tableRow("Name", "Type", primary, secondary, "Ratio", "Status", "Modified")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CFD8DC")).Render(header))
	lines = append(lines, strings.Repeat("─", maxInt(20, m.Width)))

	start := m.Offset
	end := start + listHeight
	if end > len(m.Children) {
		end = len(m.Children)
	}
	if start < 0 {
		start = 0
	}

	for i := start; i < end; i++ {
		lines = append(lines, m.renderEntry(m.Children[i], i == m.Cursor))
	}

	for len(lines) < listHeight+2 {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m AppModel) renderEntry(entry *models.Entry, selected bool) string {
	name := entry.Name
	if entry.Type == models.EntryTypeDir {
		name += "/"
	}
	nameWidth := m.nameColumnWidth()
	nameWithIcon := fmt.Sprintf("%s %s", EntryTypeIcon(entry), truncate(name, maxInt(1, nameWidth-3)))

	primaryReal := entry.PhysicalSize
	secondaryLogical := entry.LogicalSize
	if m.ShowLogical {
		primaryReal = entry.LogicalSize
		secondaryLogical = entry.PhysicalSize
	}

	ratio := fmt.Sprintf("%5s", formatPercent(entry.SizeRatio()))
	status := truncate(EntryStatusIndicator(entry), 12)
	mod := entry.ModTime.Format("2006-01-02 15:04")

	line := m.tableRow(nameWithIcon, entryTypeShort(entry.Type), FormatBytes(primaryReal), FormatBytes(secondaryLogical), ratio, status, mod)

	placeholder := entry.CloudStatus == models.CloudStatusOnlineOnly || entry.PhysicalSize == 0
	sizeColor := EntrySizeColor(entry.PhysicalSize, placeholder)
	coloredPrimary := lipgloss.NewStyle().Foreground(sizeColor).Render(FormatBytes(primaryReal))
	line = strings.Replace(line, FormatBytes(primaryReal), coloredPrimary, 1)

	if entry.Type == models.EntryTypeDir {
		line = strings.Replace(line, nameWithIcon, dirStyle.Render(nameWithIcon), 1)
	} else if entry.Type == models.EntryTypeSymlink {
		line = symlinkStyle.Render(line)
	} else if entry.IsCloud {
		line = cloudStyle.Render(line)
	}

	if selected {
		return selectedStyle.Width(maxInt(0, m.Width)).Render(line)
	}
	return normalStyle.Width(maxInt(0, m.Width)).Render(line)
}

func (m AppModel) renderSummaryPanel() string {
	if m.CurrentDir == nil || len(m.Children) == 0 {
		return ""
	}

	children := append([]*models.Entry(nil), m.Children...)
	sort.Slice(children, func(i, j int) bool {
		return children[i].PhysicalSize > children[j].PhysicalSize
	})
	if len(children) > 6 {
		children = children[:6]
	}

	maxSize := int64(1)
	for _, e := range children {
		if e.PhysicalSize > maxSize {
			maxSize = e.PhysicalSize
		}
	}

	rows := []string{lipgloss.NewStyle().Bold(true).Render("Visual Summary (top entries)")}
	barWidth := maxInt(10, minInt(40, m.Width-35))
	for _, e := range children {
		filled := int(float64(e.PhysicalSize) / float64(maxSize) * float64(barWidth))
		if filled < 0 {
			filled = 0
		}
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		name := truncate(e.Name, 18)
		rows = append(rows, fmt.Sprintf("%-18s %s %10s", name, bar, FormatBytes(e.PhysicalSize)))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#455A64")).
		Padding(0, 1).
		Render(strings.Join(rows, "\n"))
}

func (m AppModel) renderFooter() string {
	parts := make([]string, 0, 8)
	if selected := m.GetSelected(); selected != nil {
		info := fmt.Sprintf("%s | %s", selected.Path, selected.CloudProvider)
		if selected.ScanError != "" {
			info += " | " + selected.ScanError
		}
		if selected.Skipped != "" {
			info += " | " + selected.Skipped
		}
		if selected.CountedElsewhere != "" {
			info += " | counted at " + selected.CountedElsewhere
		}
		parts = append(parts, info)
	}

	sortName := "Allocated"
	switch m.SortBy {
	case models.SortByLogicalSize:
		sortName = "Logical"
	case models.SortByName:
		sortName = "Name"
	case models.SortByModTime:
		sortName = "Modified"
	case models.SortBySizeRatio:
		sortName = "Ratio"
	}
	order := "↓"
	if m.SortOrder == models.SortAscending {
		order = "↑"
	}
	parts = append(parts, fmt.Sprintf("Sort: %s %s", sortName, order))
	parts = append(parts, fmt.Sprintf("Hidden: %t", m.ShowHidden))

	parts = append(parts, "Keys: s / h l v d o t y e q")

	if len(parts) == 0 {
		return ""
	}
	first := truncate(parts[0], maxInt(1, m.Width-2))
	rest := truncate(strings.Join(parts[1:], " | "), maxInt(1, m.Width-2))
	return footerStyle.Render(first + "\n" + rest)
}

func (m AppModel) computeListHeight() int {
	height := m.Height - 11
	if m.SearchMode {
		height--
	}
	if m.ShowSummary {
		height -= 8
	}
	if height < 6 {
		height = 6
	}
	return height
}

func entryTypeShort(t models.EntryType) string {
	switch t {
	case models.EntryTypeDir:
		return "DIR"
	case models.EntryTypeSymlink:
		return "LNK"
	case models.EntryTypeFile:
		return "FILE"
	default:
		return "OTH"
	}
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func (m AppModel) nameColumnWidth() int {
	overhead := 49
	if m.Width >= 110 {
		overhead += 17
	}
	return maxInt(10, minInt(48, m.Width-overhead))
}
func (m AppModel) tableRow(name, kind, allocated, logical, ratio, status, modified string) string {
	line := fmt.Sprintf("%-*s %4s %11s %11s %6s %-12s", m.nameColumnWidth(), name, kind, allocated, logical, ratio, status)
	if m.Width >= 110 {
		line += " " + modified
	}
	return line
}

func getName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

func formatPercent(ratio float64) string {
	percent := ratio * 100
	if percent >= 999 {
		return "999+"
	}
	if percent < 0 {
		return "0"
	}
	if percent < 1 {
		return fmt.Sprintf("%.1f", percent)
	}
	return fmt.Sprintf("%.0f", percent)
}

func formatPermissions(mode uint32) string {
	perms := []byte("rwxrwxrwx")
	for i := 0; i < 9; i++ {
		if mode&(1<<(8-i)) == 0 {
			perms[i] = '-'
		}
	}
	return string(perms)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m AppModel) renderCoverage() string {
	if m.Root == nil || m.Root.Report == nil {
		return ""
	}
	r := m.Root.Report
	coverage := "Measured accessible paths"
	if m.Root.Incomplete {
		coverage = "PARTIAL scan"
	}
	line := fmt.Sprintf("%s | %d errors | %d skipped | d: diagnosis", coverage, r.Errors, r.Skipped)
	if r.Volume != nil {
		line += " | Available: " + FormatBytes(r.Volume.Available)
	}
	return truncate(line, maxInt(1, m.Width))
}
func (m *AppModel) clampDiagnosisOffset() {
	if m.Root == nil {
		m.DiagnosisOffset = 0
		return
	}
	lines := strings.Split(report.Text(m.Root), "\n")
	m.DiagnosisOffset = minInt(m.DiagnosisOffset, maxInt(0, len(lines)-maxInt(1, m.Height-3)))
}
func (m AppModel) renderDiagnosis() string {
	if m.Root == nil {
		return "No scan available. Esc to close."
	}
	lines := strings.Split(report.Text(m.Root), "\n")
	height := maxInt(1, m.Height-3)
	start := minInt(m.DiagnosisOffset, maxInt(0, len(lines)-height))
	end := minInt(len(lines), start+height)
	visible := make([]string, 0, end-start)
	for _, line := range lines[start:end] {
		visible = append(visible, truncate(line, maxInt(1, m.Width)))
	}
	return "Disk diagnosis | arrows/PgUp/PgDn: scroll | Esc/d: close\n" + strings.Join(visible, "\n")
}
