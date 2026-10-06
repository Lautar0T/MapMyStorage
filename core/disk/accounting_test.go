package disk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lautar0t/MapMyStorage/core/models"
)

func makeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0644); err != nil {
		t.Fatal(err)
	}
}
func scanTest(t *testing.T, c models.ScanConfig) *models.Entry {
	t.Helper()
	e, err := NewScanner(c).Scan()
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestAccountingDepthHiddenAndCloud(t *testing.T) {
	root := t.TempDir()
	makeFile(t, filepath.Join(root, ".cache", "big.bin"), 65536)
	makeFile(t, filepath.Join(root, "Library", "CloudStorage", "Dropbox-Team", "a", "file.bin"), 8192)
	cfg := models.ScanConfig{RootPath: root, ShowHidden: true, IncludeCloudInfo: true}
	full := scanTest(t, cfg)
	cfg.MaxDepth = 1
	shallow := scanTest(t, cfg)
	if shallow.PhysicalSize != full.PhysicalSize || shallow.LogicalSize != full.LogicalSize {
		t.Fatal("depth changed totals")
	}
	if shallow.Report.Files != 2 || shallow.Report.Directories != full.Report.Directories {
		t.Fatal("depth lost counts")
	}
	if len(shallow.Report.LargestDirectories) != int(shallow.Report.Directories) {
		t.Fatal("depth lost directory ranking")
	}
	if len(shallow.Report.LargestFiles) != 2 {
		t.Fatal("depth lost largest files")
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if c := shallow.Report.Cloud[models.CloudProviderDropbox]; c == nil || c.Files != 1 || c.Allocated <= 0 {
			t.Fatalf("cloud aggregate missing: %+v", c)
		}
	}
	for _, child := range shallow.Children {
		if len(child.Children) != 0 || !child.Summarized {
			t.Fatal("depth did not limit retained children")
		}
	}
	cfg.ShowHidden = false
	filtered := scanTest(t, cfg)
	if !filtered.Incomplete || filtered.Report.Skipped != 1 || filtered.Report.Files != 1 {
		t.Fatal("filter coverage missing")
	}
}
func TestAccountingHardLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Win32 metadata does not expose identity")
	}
	root := t.TempDir()
	a := filepath.Join(root, "a.bin")
	b := filepath.Join(root, "b.bin")
	makeFile(t, a, 8192)
	before := scanTest(t, models.ScanConfig{RootPath: root, ShowHidden: true})
	if err := os.Link(a, b); err != nil {
		t.Skip(err)
	}
	after := scanTest(t, models.ScanConfig{RootPath: root, ShowHidden: true})
	// Directory allocation can change, so compare file allocation only.
	var sum int64
	for _, e := range after.Children {
		sum += e.PhysicalSize
	}
	if sum != before.Children[0].PhysicalSize || after.Report.Duplicates != 1 {
		t.Fatalf("hard links counted twice: %+v", after.Report)
	}
	if after.Children[1].CountedElsewhere == "" {
		t.Fatal("duplicate not explained")
	}
}
func TestAccountingSymlinkCycleAndRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	makeFile(t, filepath.Join(target, "file"), 8192)
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(root, filepath.Join(target, "loop")); err != nil {
		t.Skip(err)
	}
	e := scanTest(t, models.ScanConfig{RootPath: root, ShowHidden: true, FollowSymlinks: true})
	if e.Report.Files != 1 || e.Report.Duplicates < 2 {
		t.Fatalf("cycle or duplicate not handled: %+v", e.Report)
	}
	resolved := scanTest(t, models.ScanConfig{RootPath: filepath.Join(root, "link"), ShowHidden: true})
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Type != models.EntryTypeDir || resolved.Path != realTarget {
		t.Fatal("explicit symlink root not resolved")
	}
}
func TestAccountingPermissionErrorVisible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix mode permissions")
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	makeFile(t, filepath.Join(blocked, "file"), 8192)
	if err := os.Chmod(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blocked, 0755)
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("current user bypasses mode permissions")
	}
	for _, depth := range []int{0, 1} {
		e := scanTest(t, models.ScanConfig{RootPath: root, ShowHidden: true, MaxDepth: depth})
		if !e.Incomplete || e.Report.Errors != 1 || len(e.Report.Issues) != 1 || e.Children[0].ScanError == "" {
			t.Fatal("permission error concealed")
		}
	}
}
func TestCancellationAndAsyncParity(t *testing.T) {
	root := t.TempDir()
	makeFile(t, filepath.Join(root, "file"), 4096)
	s := NewScanner(models.ScanConfig{RootPath: root, ShowHidden: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ScanWithContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error: %v", err)
	}
	expected, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.ScanAsync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got *models.Entry
	for ev := range ch {
		if ev.Type == models.ScanEventError {
			t.Fatal(ev.Error)
		}
		if ev.Type == models.ScanEventCompleted {
			got = ev.Entry
		}
	}
	if got == nil || got.PhysicalSize != expected.PhysicalSize || got.Report.Files != expected.Report.Files {
		t.Fatal("async differs from sync")
	}
	// An abandoned progress consumer must not keep the scanner alive after cancel.
	ctx, cancel = context.WithCancel(context.Background())
	ch, _ = s.ScanAsync(ctx)
	cancel()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("canceled channel remained open")
		}
	}
}
func TestCloudPathHints(t *testing.T) {
	cases := map[string]models.CloudProvider{
		"/Users/me/Library/CloudStorage/Dropbox-Team/a.bin":               models.CloudProviderDropbox,
		"/Users/me/Library/CloudStorage/OneDrive-Personal/a.bin":          models.CloudProviderOneDrive,
		"/Users/me/Library/CloudStorage/GoogleDrive-me@example.com/a.bin": models.CloudProviderGoogleDrive,
		"/Users/me/Library/Mobile Documents/com~apple~CloudDocs/a.bin":    models.CloudProviderICloud,
		"/Users/me/Library/Application Support/Dropbox/cache.bin":         models.CloudProviderNone,
		"/Users/me/Library/CloudStorage/Dropbox-Team/Caches/file":         models.CloudProviderDropbox,
		"/tmp/my-dropbox-report.pdf":                                      models.CloudProviderNone,
		"/tmp/dropbox-backup.zip":                                         models.CloudProviderNone,
	}
	for path, want := range cases {
		if got := GetCloudProviderFromPath(path); got != want {
			t.Errorf("%s got %s, want %s", path, got, want)
		}
	}
}
