package disk

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/lautar0t/MapMyStorage/core/models"
)

type fakeDatalessInfo struct{ directory bool }

func (f fakeDatalessInfo) Name() string { return "placeholder" }
func (f fakeDatalessInfo) Size() int64  { return 1024 * 1024 }
func (f fakeDatalessInfo) Mode() os.FileMode {
	if f.directory {
		return os.ModeDir | 0755
	}
	return 0644
}
func (f fakeDatalessInfo) ModTime() time.Time { return time.Time{} }
func (f fakeDatalessInfo) IsDir() bool        { return f.directory }
func (f fakeDatalessInfo) Sys() any           { return &syscall.Stat_t{Flags: 0x40000000} }
func TestDatalessEvidence(t *testing.T) {
	provider, status, err := NewCloudDetector().DetectCloudStatus("/Users/me/Library/CloudStorage/Dropbox/file", fakeDatalessInfo{})
	if err != nil || provider != models.CloudProviderDropbox || status != models.CloudStatusOnlineOnly {
		t.Fatalf("%s %s %v", provider, status, err)
	}
}
func TestDatalessDirectoryNotEnumerated(t *testing.T) {
	// A non-existent path proves visit did not try os.Open/ReadDir.
	s := NewScanner(models.ScanConfig{IncludeCloudInfo: true})
	st := &scanState{report: &models.ScanReport{Cloud: make(map[models.CloudProvider]*models.CloudUsage)}, devices: map[string]bool{"0": true}, seen: map[string]string{}}
	e, err := s.visit(t.Context(), "/nonexistent/Dropbox/dir", fakeDatalessInfo{directory: true}, 0, st)
	if err != nil || e.Skipped == "" || e.ScanError != "" || !e.Incomplete || st.report.Errors != 0 {
		t.Fatalf("dataless directory enumerated: %+v %v", e, err)
	}
}

func TestDiagnosticVolumeForSubdirectoryAndFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	dirVolume, err := diagnosticVolume(root)
	if err != nil {
		t.Fatal(err)
	}
	fileVolume, err := diagnosticVolume(path)
	if err != nil {
		t.Fatal(err)
	}
	if dirVolume != fileVolume || dirVolume == root || !filepath.IsAbs(dirVolume) {
		t.Fatalf("not a containing mount point: %q %q", dirVolume, fileVolume)
	}
	mount, err := os.Stat(dirVolume)
	if err != nil || !mount.IsDir() {
		t.Fatalf("unusable mount point: %v", err)
	}
	file, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if deviceIdentity(mount) != deviceIdentity(file) {
		t.Fatal("resolved a different filesystem")
	}
}
