//go:build !windows

package disk

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/lautar0t/MapMyStorage/core/models"
)

func TestZeroBlocksStayZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sparse.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(64 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := info.Sys().(*syscall.Stat_t).Blocks * 512
	got, err := GetPhysicalSize(path, info)
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("invented allocation: got %d, stat says %d", got, expected)
	}
}
func TestDirectoryAllocationIncluded(t *testing.T) {
	root := t.TempDir()
	makeFile(t, filepath.Join(root, "nested", "file.bin"), 8192)
	var expected int64
	err := filepath.Walk(root, func(path string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		expected += i.Sys().(*syscall.Stat_t).Blocks * 512
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	e := scanTest(t, models.ScanConfig{RootPath: root, ShowHidden: true})
	if e.TotalPhysicalSize() != expected {
		t.Fatalf("got %d, stat sum %d", e.PhysicalSize, expected)
	}
}
func TestFilesystemBoundary(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	s := NewScanner(models.ScanConfig{ShowHidden: true})
	state := &scanState{report: &models.ScanReport{}, devices: map[string]bool{}, seen: map[string]string{}}
	e, err := s.visit(t.Context(), root, info, 0, state)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Incomplete || e.Skipped == "" || e.PhysicalSize != 0 {
		t.Fatal("mount boundary not respected")
	}
}
