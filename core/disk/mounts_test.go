package disk

import (
	"github.com/lautar0t/MapMyStorage/core/models"
	"os"
	"path/filepath"
	"testing"
)

func TestKnownMountSkippedBeforeTraversal(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "mounted")
	makeFile(t, filepath.Join(childPath, "large.bin"), 65536)
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	s := NewScanner(models.ScanConfig{ShowHidden: true})
	state := &scanState{report: &models.ScanReport{}, seen: map[string]string{}, devices: map[string]bool{deviceIdentity(info): true}, mounts: map[string]bool{childPath: true}}
	e, err := s.visit(t.Context(), root, info, 0, state)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Incomplete || state.report.Files != 0 || state.report.Skipped != 1 || len(e.Children) != 1 || e.Children[0].Skipped == "" {
		t.Fatal("entered excluded mount")
	}
}
