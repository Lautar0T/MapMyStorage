// Package disk provides filesystem scanning and disk usage calculation.
package disk

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lautaro/real-disk-map/core/models"
)

// Scanner scans directories and calculates real disk usage.
type Scanner struct {
	config         models.ScanConfig
	cloudDetector  *CloudDetector
	excludeRegexps []*regexp.Regexp
}

// NewScanner creates a new scanner with the given configuration.
func NewScanner(config models.ScanConfig) *Scanner {
	s := &Scanner{
		config:        config,
		cloudDetector: NewCloudDetector(),
	}

	for _, pattern := range config.ExcludePatterns {
		if pattern == "" {
			continue
		}
		re, err := compilePattern(pattern)
		if err == nil {
			s.excludeRegexps = append(s.excludeRegexps, re)
		}
	}

	return s
}

// compilePattern compiles a pattern into a regexp.
// Supports glob-like patterns such as *.log and node_modules.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	// Match either separator style so one config works on macOS and Windows.
	return regexp.Compile(`(?:^|[/\\])` + escaped + `$`)
}

// Scan performs a full scan of the configured root path.
func (s *Scanner) Scan() (*models.Entry, error) {
	return s.ScanWithContext(context.Background())
}

// ScanWithContext performs a scan with context cancellation support.
func (s *Scanner) ScanWithContext(ctx context.Context) (*models.Entry, error) {
	rootPath, err := filepath.Abs(s.config.RootPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return s.scanFile(rootPath, info)
	}

	rootEntry := s.newDirectoryEntry(rootPath, info)
	visited := map[string]struct{}{}
	if s.config.FollowSymlinks {
		visited[s.realPath(rootPath)] = struct{}{}
	}

	if err := s.scanDirectory(ctx, rootEntry, 0, visited, nil); err != nil {
		return nil, err
	}

	return rootEntry, nil
}

// ScanAsync performs a scan and returns progress/results through a channel.
func (s *Scanner) ScanAsync(ctx context.Context) (<-chan models.ScanEvent, error) {
	eventCh := make(chan models.ScanEvent, 128)

	go func() {
		defer close(eventCh)

		eventCh <- models.ScanEvent{Type: models.ScanEventStarted}

		var filesScanned, dirsScanned, bytesProcessed, errorsEncountered int64

		emit := func(path string) {
			processed := atomic.LoadInt64(&filesScanned) + atomic.LoadInt64(&dirsScanned)
			if processed%200 != 0 {
				return
			}

			eventCh <- models.ScanEvent{
				Type: models.ScanEventProgress,
				Progress: &models.ScanProgress{
					FilesScanned:      atomic.LoadInt64(&filesScanned),
					DirsScanned:       atomic.LoadInt64(&dirsScanned),
					BytesProcessed:    atomic.LoadInt64(&bytesProcessed),
					CurrentPath:       path,
					ErrorsEncountered: atomic.LoadInt64(&errorsEncountered),
				},
			}
		}

		rootPath, err := filepath.Abs(s.config.RootPath)
		if err != nil {
			eventCh <- models.ScanEvent{Type: models.ScanEventError, Error: err}
			return
		}

		info, err := os.Lstat(rootPath)
		if err != nil {
			eventCh <- models.ScanEvent{Type: models.ScanEventError, Error: err}
			return
		}

		if !info.IsDir() {
			entry, scanErr := s.scanFile(rootPath, info)
			if scanErr != nil {
				eventCh <- models.ScanEvent{Type: models.ScanEventError, Error: scanErr}
				return
			}
			eventCh <- models.ScanEvent{Type: models.ScanEventCompleted, Entry: entry}
			return
		}

		rootEntry := s.newDirectoryEntry(rootPath, info)
		visited := map[string]struct{}{}
		if s.config.FollowSymlinks {
			visited[s.realPath(rootPath)] = struct{}{}
		}

		progress := &scanProgressState{
			filesScanned:      &filesScanned,
			dirsScanned:       &dirsScanned,
			bytesProcessed:    &bytesProcessed,
			errorsEncountered: &errorsEncountered,
			emit:              emit,
		}

		if err := s.scanDirectory(ctx, rootEntry, 0, visited, progress); err != nil {
			eventCh <- models.ScanEvent{Type: models.ScanEventError, Error: err}
			return
		}

		eventCh <- models.ScanEvent{
			Type: models.ScanEventProgress,
			Progress: &models.ScanProgress{
				FilesScanned:      atomic.LoadInt64(&filesScanned),
				DirsScanned:       atomic.LoadInt64(&dirsScanned),
				BytesProcessed:    atomic.LoadInt64(&bytesProcessed),
				CurrentPath:       rootPath,
				ErrorsEncountered: atomic.LoadInt64(&errorsEncountered),
			},
		}
		eventCh <- models.ScanEvent{Type: models.ScanEventCompleted, Entry: rootEntry}
	}()

	return eventCh, nil
}

type scanProgressState struct {
	filesScanned      *int64
	dirsScanned       *int64
	bytesProcessed    *int64
	errorsEncountered *int64
	emit              func(path string)
}

func (s *Scanner) scanDirectory(
	ctx context.Context,
	parent *models.Entry,
	depth int,
	visited map[string]struct{},
	progress *scanProgressState,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if progress != nil {
		atomic.AddInt64(progress.dirsScanned, 1)
		progress.emit(parent.Path)
	}

	if s.config.MaxDepth > 0 && depth >= s.config.MaxDepth {
		logical, physical := s.summarizeDirectory(ctx, parent.Path, visited, progress)
		parent.LogicalSize = logical
		parent.PhysicalSize = physical
		parent.Children = nil
		return nil
	}

	entries, err := os.ReadDir(parent.Path)
	if err != nil {
		if progress != nil {
			atomic.AddInt64(progress.errorsEncountered, 1)
		}
		return nil
	}

	semaphore := make(chan struct{}, runtime.NumCPU()*2)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, dirEntry := range entries {
		name := dirEntry.Name()
		path := filepath.Join(parent.Path, name)

		if !s.config.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if s.isExcluded(path, name) {
			continue
		}

		info, err := os.Lstat(path)
		if err != nil {
			if progress != nil {
				atomic.AddInt64(progress.errorsEncountered, 1)
			}
			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		treatAsDir := info.IsDir()
		if isSymlink && s.config.FollowSymlinks {
			targetInfo, targetErr := os.Stat(path)
			if targetErr == nil && targetInfo.IsDir() {
				treatAsDir = true
				info = targetInfo
			}
		}

		if treatAsDir {
			child := s.newDirectoryEntry(path, info)
			child.Parent = parent
			child.IsSymlink = isSymlink

			if s.config.FollowSymlinks {
				real := s.realPath(path)
				if _, exists := visited[real]; exists {
					continue
				}
				visited[real] = struct{}{}
			}

			err := s.scanDirectory(ctx, child, depth+1, visited, progress)
			if s.config.FollowSymlinks {
				delete(visited, s.realPath(path))
			}
			if err != nil {
				if progress != nil {
					atomic.AddInt64(progress.errorsEncountered, 1)
				}
				continue
			}

			mu.Lock()
			parent.Children = append(parent.Children, child)
			mu.Unlock()
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}
		go func(path string, info os.FileInfo) {
			defer wg.Done()
			defer func() { <-semaphore }()

			child, scanErr := s.scanFile(path, info)
			if scanErr != nil {
				if progress != nil {
					atomic.AddInt64(progress.errorsEncountered, 1)
				}
				return
			}
			child.Parent = parent

			mu.Lock()
			parent.Children = append(parent.Children, child)
			mu.Unlock()

			if progress != nil {
				atomic.AddInt64(progress.filesScanned, 1)
				atomic.AddInt64(progress.bytesProcessed, child.PhysicalSize)
				progress.emit(path)
			}
		}(path, info)
	}

	wg.Wait()

	sort.Sort(models.ByPhysicalSize(parent.Children))
	var totalLogical, totalPhysical int64
	for _, child := range parent.Children {
		totalLogical += child.TotalLogicalSize()
		totalPhysical += child.TotalPhysicalSize()
	}
	parent.LogicalSize = totalLogical
	parent.PhysicalSize = totalPhysical

	return nil
}

func (s *Scanner) summarizeDirectory(
	ctx context.Context,
	path string,
	visited map[string]struct{},
	progress *scanProgressState,
) (int64, int64) {
	if err := ctx.Err(); err != nil {
		return 0, 0
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		if progress != nil {
			atomic.AddInt64(progress.errorsEncountered, 1)
		}
		return 0, 0
	}

	var logicalTotal, physicalTotal int64
	for _, dirEntry := range entries {
		name := dirEntry.Name()
		childPath := filepath.Join(path, name)

		if !s.config.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if s.isExcluded(childPath, name) {
			continue
		}

		info, lerr := os.Lstat(childPath)
		if lerr != nil {
			if progress != nil {
				atomic.AddInt64(progress.errorsEncountered, 1)
			}
			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		treatAsDir := info.IsDir()
		if isSymlink && s.config.FollowSymlinks {
			targetInfo, targetErr := os.Stat(childPath)
			if targetErr == nil && targetInfo.IsDir() {
				treatAsDir = true
				info = targetInfo
			}
		}

		if treatAsDir {
			if s.config.FollowSymlinks {
				real := s.realPath(childPath)
				if _, exists := visited[real]; exists {
					continue
				}
				visited[real] = struct{}{}
				subLogical, subPhysical := s.summarizeDirectory(ctx, childPath, visited, progress)
				delete(visited, real)
				logicalTotal += subLogical
				physicalTotal += subPhysical
			} else {
				subLogical, subPhysical := s.summarizeDirectory(ctx, childPath, visited, progress)
				logicalTotal += subLogical
				physicalTotal += subPhysical
			}
			continue
		}

		logicalTotal += info.Size()
		physicalSize, perr := GetPhysicalSize(childPath, info)
		if perr != nil {
			physicalSize = 0
		}
		physicalTotal += physicalSize
		if progress != nil {
			atomic.AddInt64(progress.filesScanned, 1)
			atomic.AddInt64(progress.bytesProcessed, physicalSize)
			progress.emit(childPath)
		}
	}

	return logicalTotal, physicalTotal
}

// scanFile scans a single file/symlink and returns its entry.
func (s *Scanner) scanFile(path string, info os.FileInfo) (*models.Entry, error) {
	entryType := models.EntryTypeFile
	isSymlink := info.Mode()&os.ModeSymlink != 0
	if isSymlink {
		entryType = models.EntryTypeSymlink
	}

	logicalSize := info.Size()
	physicalSize, err := GetPhysicalSize(path, info)
	if err != nil {
		physicalSize = 0
	}

	clusterSize, _ := GetClusterSize(filepath.Dir(path))
	uid, gid := extractOwnership(info)

	provider := models.CloudProviderNone
	status := models.CloudStatusLocal
	isCloud := false
	if s.config.IncludeCloudInfo {
		provider, status, _ = s.cloudDetector.DetectCloudStatus(path, info)
		isCloud = provider != models.CloudProviderNone
	}

	return &models.Entry{
		Path:          path,
		Name:          info.Name(),
		Type:          entryType,
		LogicalSize:   logicalSize,
		PhysicalSize:  physicalSize,
		ClusterSize:   clusterSize,
		IsSymlink:     isSymlink,
		IsCloud:       isCloud,
		CloudProvider: provider,
		CloudStatus:   status,
		ModTime:       info.ModTime(),
		Permissions:   uint32(info.Mode().Perm()),
		UID:           uid,
		GID:           gid,
	}, nil
}

func (s *Scanner) newDirectoryEntry(path string, info os.FileInfo) *models.Entry {
	uid, gid := extractOwnership(info)

	return &models.Entry{
		Path:         path,
		Name:         filepath.Base(path),
		Type:         models.EntryTypeDir,
		LogicalSize:  0,
		PhysicalSize: 0,
		ModTime:      info.ModTime(),
		Permissions:  uint32(info.Mode().Perm()),
		UID:          uid,
		GID:          gid,
		Children:     make([]*models.Entry, 0),
	}
}

func (s *Scanner) realPath(path string) string {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(real)
}

// isExcluded checks whether a path or base name matches any configured pattern.
func (s *Scanner) isExcluded(path, name string) bool {
	normPath := strings.ReplaceAll(path, `\\`, `/`)
	for _, re := range s.excludeRegexps {
		if re.MatchString(normPath) || re.MatchString(name) {
			return true
		}
	}
	return false
}

// GetEntryChildren returns the children of a directory entry, sorted by options.
func GetEntryChildren(entry *models.Entry, opts models.SortOptions, showHidden bool) []*models.Entry {
	if entry == nil || entry.Type != models.EntryTypeDir || len(entry.Children) == 0 {
		return nil
	}

	children := make([]*models.Entry, 0, len(entry.Children))
	for _, child := range entry.Children {
		if showHidden || !child.IsHidden() {
			children = append(children, child)
		}
	}

	models.SortEntries(children, opts)
	return children
}

// WalkEntry is kept as compatibility wrapper for older callers.
func WalkEntry(entry *models.Entry, callback func(*models.Entry) error) error {
	if entry == nil {
		return nil
	}
	if err := callback(entry); err != nil {
		if errors.Is(err, fs.SkipDir) {
			return nil
		}
		return err
	}
	for _, child := range entry.Children {
		if err := WalkEntry(child, callback); err != nil {
			return err
		}
	}
	return nil
}

// FindEntry is kept as compatibility wrapper for older callers.
func FindEntry(root *models.Entry, path string) *models.Entry {
	if root == nil {
		return nil
	}
	if root.Path == path {
		return root
	}
	for _, child := range root.Children {
		if found := FindEntry(child, path); found != nil {
			return found
		}
	}
	return nil
}
