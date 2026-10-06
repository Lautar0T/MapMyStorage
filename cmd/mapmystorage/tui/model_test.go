package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lautar0t/MapMyStorage/core/models"
)

func TestSizeColumnLabelsFollowValues(t *testing.T) {
	m := NewAppModel(Config{})
	defer m.ScanCancel()
	m.Width = 120
	m.Height = 30
	m.ShowLogical = true
	line := m.renderFileList()
	if strings.Index(line, "Logical") >= strings.Index(line, "Allocated") {
		t.Fatal("logical values mislabeled as allocated")
	}
}
func TestSearchQDoesNotQuit(t *testing.T) {
	m := NewAppModel(Config{})
	defer m.ScanCancel()
	m.SearchMode = true
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil || updated.(AppModel).SearchQuery != "q" {
		t.Fatal("q quit instead of searching")
	}
}
func TestQuitCancelsScan(t *testing.T) {
	m := NewAppModel(Config{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil || m.Config.Context.Err() != context.Canceled {
		t.Fatal("quit did not cancel scanner")
	}
}
func TestDiagnosisAndCoverage(t *testing.T) {
	m := NewAppModel(Config{})
	defer m.ScanCancel()
	m.Scanning = false
	m.Width = 100
	m.Height = 24
	m.Root = &models.Entry{Path: "/", Incomplete: true, Report: &models.ScanReport{Errors: 3}}
	m.CurrentDir = m.Root
	if !strings.Contains(m.View(), "PARTIAL") {
		t.Fatal("partial scan concealed")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(AppModel)
	if !m.Diagnosis || !strings.Contains(m.View(), "Disk diagnosis") {
		t.Fatal("diagnosis unavailable")
	}
}

func TestEnterCountedDirectoryNavigatesOriginal(t *testing.T) {
	original := &models.Entry{Path: "/System/Volumes/Data/Users", Type: models.EntryTypeDir}
	alias := &models.Entry{Path: "/Users", Type: models.EntryTypeDir, CountedElsewhere: original.Path}
	root := &models.Entry{Path: "/", Type: models.EntryTypeDir, Children: []*models.Entry{alias, original}}
	m := NewAppModel(Config{})
	defer m.ScanCancel()
	m.Scanning = false
	m.Root = root
	m.CurrentDir = root
	m.Children = []*models.Entry{alias}
	m.Cursor = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(AppModel).CurrentDir != original {
		t.Fatal("duplicate alias opened an empty directory")
	}
}
