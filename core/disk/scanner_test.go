package disk

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lautar0t/MapMyStorage/core/models"
)

func TestScanner_Scan(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test files
	files := map[string]int64{
		"file1.txt":       1000,
		"file2.txt":       2000,
		"subdir/file.txt": 500,
	}

	for name, size := range files {
		path := filepath.Join(tmpDir, name)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		content := make([]byte, size)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	config := models.ScanConfig{
		RootPath:         tmpDir,
		FollowSymlinks:   false,
		ExcludePatterns:  nil,
		MaxDepth:         0,
		ShowHidden:       true,
		IncludeCloudInfo: false,
	}

	scanner := NewScanner(config)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.Type != models.EntryTypeDir {
		t.Errorf("Expected directory type, got %s", result.Type)
	}

	// Check that we found files
	if len(result.Children) == 0 {
		t.Error("No children found")
	}
}

func TestScanner_ExcludePatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-exclude-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files
	os.WriteFile(filepath.Join(tmpDir, "keep.txt"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "exclude.log"), []byte("data"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "node_modules", "file.js"), []byte("data"), 0644)

	config := models.ScanConfig{
		RootPath:         tmpDir,
		FollowSymlinks:   false,
		ExcludePatterns:  []string{"*.log", "node_modules"},
		MaxDepth:         0,
		ShowHidden:       true,
		IncludeCloudInfo: false,
	}

	scanner := NewScanner(config)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Check that excluded files are not present
	for _, child := range result.Children {
		if child.Name == "exclude.log" {
			t.Error("exclude.log should be excluded")
		}
		if child.Name == "node_modules" {
			t.Error("node_modules should be excluded")
		}
	}

	// Check that keep.txt is present
	foundKeep := false
	for _, child := range result.Children {
		if child.Name == "keep.txt" {
			foundKeep = true
			break
		}
	}
	if !foundKeep {
		t.Error("keep.txt should be present")
	}
}

func TestScanner_MaxDepthSummariesContributeToTotals(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-max-depth-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}
	nestedFile := filepath.Join(nestedDir, "file.bin")
	if err := os.WriteFile(nestedFile, make([]byte, 16*1024), 0o644); err != nil {
		t.Fatalf("Failed to write nested file: %v", err)
	}

	scanner := NewScanner(models.ScanConfig{
		RootPath:         tmpDir,
		MaxDepth:         1,
		ShowHidden:       true,
		IncludeCloudInfo: false,
	})

	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if got := result.TotalPhysicalSize(); got <= 0 {
		t.Fatalf("Expected max-depth summary to contribute physical size, got %d", got)
	}
	if got := result.TotalLogicalSize(); got <= 0 {
		t.Fatalf("Expected max-depth summary to contribute logical size, got %d", got)
	}
}

func TestGetPhysicalSize_ValidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "phys-size-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file with known size
	content := make([]byte, 4096)
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	physical, err := GetPhysicalSize(path, info)
	if err != nil {
		t.Fatalf("GetPhysicalSize failed: %v", err)
	}

	// Physical size should be close to logical size for regular files
	if physical == 0 {
		t.Error("Physical size is 0")
	}

	if physical > info.Size()*2 {
		t.Errorf("Physical size too large: %d > %d", physical, info.Size())
	}
}

func TestIsCloudPlaceholder(t *testing.T) {
	cd := NewCloudDetector()

	tmpDir, err := os.MkdirTemp("", "cloud-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a regular file
	path := filepath.Join(tmpDir, "regular.txt")
	os.WriteFile(path, []byte("data"), 0644)

	info, _ := os.Stat(path)

	// Regular file should not be a placeholder
	if cd.IsCloudPlaceholder(path, info) {
		t.Error("Regular file should not be cloud placeholder")
	}
}

func TestScanner_DoesNotFollowSymlinkByDefault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-symlink-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}
	targetFile := filepath.Join(targetDir, "large.bin")
	if err := os.WriteFile(targetFile, make([]byte, 32*1024), 0o644); err != nil {
		t.Fatalf("Failed to write target file: %v", err)
	}

	symlinkPath := filepath.Join(tmpDir, "link-to-target")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skipf("symlink unsupported in environment: %v", err)
	}

	scanner := NewScanner(models.ScanConfig{
		RootPath:         tmpDir,
		FollowSymlinks:   false,
		IncludeCloudInfo: false,
		ShowHidden:       true,
	})

	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var linkEntry *models.Entry
	for _, child := range result.Children {
		if child.Name == "link-to-target" {
			linkEntry = child
			break
		}
	}
	if linkEntry == nil {
		t.Fatalf("Expected symlink entry in scan result")
	}
	if linkEntry.Type != models.EntryTypeSymlink {
		t.Fatalf("Expected symlink type, got %s", linkEntry.Type)
	}
	if len(linkEntry.Children) != 0 {
		t.Fatalf("Symlink should not have scanned children when FollowSymlinks=false")
	}
}

func TestGetPhysicalSize_SparseFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sparse behavior test is platform-specific and skipped on windows in this suite")
	}

	tmpDir, err := os.MkdirTemp("", "sparse-size-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "sparse.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Write a small block at start and another far offset to create holes.
	if _, err := f.Write(make([]byte, 4096)); err != nil {
		f.Close()
		t.Fatalf("Failed writing first block: %v", err)
	}
	if _, err := f.Seek(64*1024*1024, 0); err != nil {
		f.Close()
		t.Fatalf("Failed seeking: %v", err)
	}
	if _, err := f.Write(make([]byte, 4096)); err != nil {
		f.Close()
		t.Fatalf("Failed writing second block: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	physical, err := GetPhysicalSize(path, info)
	if err != nil {
		t.Fatalf("GetPhysicalSize failed: %v", err)
	}
	if physical <= 0 {
		t.Fatalf("Physical size should be > 0")
	}
	if physical >= info.Size() {
		t.Fatalf("Expected sparse allocated size < logical size, got physical=%d logical=%d", physical, info.Size())
	}
}

func TestCloudDetector_InferOnlineOnlyFromSparseRatio(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sparse ratio simulation is flaky on windows in CI")
	}

	tmpDir, err := os.MkdirTemp("", "cloud-sim-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "placeholder-sim.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	if _, err := f.Seek(128*1024*1024, 0); err != nil {
		f.Close()
		t.Fatalf("Failed to seek: %v", err)
	}
	if _, err := f.Write([]byte{1}); err != nil {
		f.Close()
		t.Fatalf("Failed to write trailing byte: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	status := NewCloudDetector().inferStatusFromSize(path, info)
	if status != models.CloudStatusOnlineOnly {
		t.Fatalf("Expected simulated online-only status, got %s", status)
	}
}
