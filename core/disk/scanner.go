// Package disk scans filesystem metadata without opening file contents.
package disk

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lautar0t/MapMyStorage/core/models"
)

type Scanner struct {
	config         models.ScanConfig
	cloudDetector  *CloudDetector
	excludeRegexps []*regexp.Regexp
}

func NewScanner(config models.ScanConfig) *Scanner {
	s := &Scanner{config: config, cloudDetector: NewCloudDetector()}
	for _, pattern := range config.ExcludePatterns {
		if pattern == "" {
			continue
		}
		if re, err := compilePattern(pattern); err == nil {
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

// Every scan owns its state; Scanner is safe to reuse concurrently.
type scanState struct {
	report       *models.ScanReport
	seen         map[string]string
	devices      map[string]bool
	mounts       map[string]bool
	clusterSizes map[string]int64
	progress     func(models.ScanProgress)
	lastProgress time.Time
	bytes        int64
}

func (s *Scanner) Scan() (*models.Entry, error) { return s.ScanWithContext(context.Background()) }
func (s *Scanner) ScanWithContext(ctx context.Context) (*models.Entry, error) {
	return s.scan(ctx, nil)
}

func (s *Scanner) ScanAsync(ctx context.Context) (<-chan models.ScanEvent, error) {
	ch := make(chan models.ScanEvent, 16)
	go func() {
		defer close(ch)
		send := func(e models.ScanEvent) {
			select {
			case ch <- e:
			case <-ctx.Done():
			}
		}
		send(models.ScanEvent{Type: models.ScanEventStarted})
		root, err := s.scan(ctx, func(p models.ScanProgress) {
			// Progress may be dropped; completion and errors must not be dropped.
			select {
			case ch <- models.ScanEvent{Type: models.ScanEventProgress, Progress: &p}:
			default:
			}
		})
		if err != nil {
			send(models.ScanEvent{Type: models.ScanEventError, Error: err})
			return
		}
		send(models.ScanEvent{Type: models.ScanEventCompleted, Entry: root})
	}()
	return ch, nil
}

func (s *Scanner) scan(ctx context.Context, progress func(models.ScanProgress)) (*models.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(s.config.RootPath)
	if err != nil {
		return nil, err
	}
	// Resolve the explicitly requested root (e.g. ~/Dropbox), not descendant links.
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	report := &models.ScanReport{StartedAt: time.Now(), Config: s.config, Cloud: make(map[models.CloudProvider]*models.CloudUsage)}
	report.Config.RootPath = path
	report.Volume, err = VolumeUsage(path)
	if err != nil {
		report.VolumeError = err.Error()
	}
	st := &scanState{report: report, seen: make(map[string]string), devices: map[string]bool{deviceIdentity(info): true}, progress: progress}
	if s.config.WholeDisk && runtime.GOOS == "darwin" {
		// Startup System, Data and VM volumes only. Recovery, backups, external and
		// network mounts stay outside the scan unless explicitly requested.
		for _, mount := range []string{"/System/Volumes/Data", "/System/Volumes/VM"} {
			if i, e := os.Stat(mount); e == nil {
				st.devices[deviceIdentity(i)] = true
			}
		}
	}
	var mountErr error
	if !s.config.CrossFilesystems {
		st.mounts, mountErr = excludedMounts(path, s.config.WholeDisk)
	}
	root, err := s.visit(ctx, path, info, 0, st)
	if err != nil {
		return nil, err
	}
	if mountErr != nil {
		st.issue(root, "mount table unavailable: "+mountErr.Error(), true)
	}
	if s.config.Diagnostics || s.config.WholeDisk {
		report.Diagnostics = CollectDiagnostics(ctx, path)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	report.FinishedAt = time.Now()
	root.Report = report
	st.emit(path, true)
	return root, nil
}

func (st *scanState) emit(path string, force bool) {
	if st.progress == nil || (!force && time.Since(st.lastProgress) < 100*time.Millisecond) {
		return
	}
	st.lastProgress = time.Now()
	st.progress(models.ScanProgress{FilesScanned: st.report.Files, DirsScanned: st.report.Directories, BytesProcessed: st.bytes, CurrentPath: path, ErrorsEncountered: st.report.Errors})
}
func (st *scanState) issue(e *models.Entry, reason string, isError bool) {
	e.Incomplete = true
	if isError {
		e.ScanError = reason
		st.report.Errors++
	} else {
		e.Skipped = reason
		st.report.Skipped++
	}
	if len(st.report.Issues) < 100 {
		st.report.Issues = append(st.report.Issues, models.ScanIssue{Path: e.Path, Reason: reason})
	}
}

func (s *Scanner) visit(ctx context.Context, path string, info os.FileInfo, depth int, st *scanState) (*models.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	uid, gid := extractOwnership(info)
	e := &models.Entry{Path: path, Name: filepath.Base(path), ModTime: info.ModTime(), Permissions: uint32(info.Mode().Perm()), UID: uid, GID: gid}
	e.IsSymlink = info.Mode()&os.ModeSymlink != 0
	if e.IsSymlink {
		e.SymlinkTarget, _ = os.Readlink(path)
		if s.config.FollowSymlinks {
			target, err := os.Stat(path)
			if err != nil {
				st.issue(e, err.Error(), true)
			} else {
				info = target
			}
		}
	}
	switch {
	case info.IsDir():
		e.Type = models.EntryTypeDir
	case info.Mode()&os.ModeSymlink != 0:
		e.Type = models.EntryTypeSymlink
	case info.Mode().IsRegular():
		e.Type = models.EntryTypeFile
	default:
		e.Type = models.EntryTypeOther
	}
	if e.Type != models.EntryTypeDir {
		e.LogicalSize = info.Size()
	}
	if !s.config.CrossFilesystems && !st.devices[deviceIdentity(info)] {
		st.issue(e, "different filesystem (use -cross-filesystems to include)", false)
		return e, nil
	}
	// Device + inode catches hard links, firmlinks and followed symlink cycles.
	id := ""
	if info.IsDir() || hasMultipleLinks(info) || s.config.FollowSymlinks {
		id = fileIdentity(info)
	}
	if id == "" && info.IsDir() {
		id, _ = filepath.EvalSymlinks(path)
	}
	if id != "" {
		if original, ok := st.seen[id]; ok {
			e.CountedElsewhere = original
			st.report.Duplicates++
			return e, nil
		}
		st.seen[id] = path
	}
	if st.clusterSizes == nil {
		st.clusterSizes = make(map[string]int64)
	}
	device := deviceIdentity(info)
	if device == "" {
		device = filepath.VolumeName(path)
	}
	cluster, known := st.clusterSizes[device]
	if !known {
		clusterPath := path
		if e.Type != models.EntryTypeDir {
			clusterPath = filepath.Dir(path)
		}
		cluster, _ = GetClusterSize(clusterPath)
		st.clusterSizes[device] = cluster
	}
	e.ClusterSize = cluster
	physical, err := GetPhysicalSize(path, info)
	if err != nil {
		st.issue(e, err.Error(), true)
	} else {
		e.PhysicalSize = physical
	}
	if s.config.IncludeCloudInfo && !e.IsSymlink {
		e.CloudProvider, e.CloudStatus, _ = s.cloudDetector.DetectCloudStatus(path, info)
		e.IsCloud = e.CloudProvider != models.CloudProviderNone
	}
	if e.Type != models.EntryTypeDir {
		st.report.Files++
		st.bytes += e.PhysicalSize
		if e.IsCloud {
			c := st.report.Cloud[e.CloudProvider]
			if c == nil {
				c = &models.CloudUsage{}
				st.report.Cloud[e.CloudProvider] = c
			}
			c.Files++
			c.Logical += e.LogicalSize
			c.Allocated += e.PhysicalSize
			if e.CloudStatus == models.CloudStatusOnlineOnly {
				c.OnlineOnly++
			}
			if e.CloudStatus == models.CloudStatusUnknown {
				c.Unknown++
			}
		}
		if e.Type == models.EntryTypeFile {
			st.keepLargest(e)
		}
		st.emit(path, false)
		return e, nil
	}
	defer func() { st.report.LargestDirectories = keepLargest(st.report.LargestDirectories, e) }()
	st.report.Directories++
	st.bytes += e.PhysicalSize
	st.emit(path, false)
	// Enumerating a dataless directory can cause macOS to materialize it.
	if isDataless(info) {
		if e.IsCloud {
			c := st.report.Cloud[e.CloudProvider]
			if c == nil {
				c = &models.CloudUsage{}
				st.report.Cloud[e.CloudProvider] = c
			}
			c.SkippedDirectories++
		}
		st.issue(e, "online-only directory: not enumerated to avoid materialization", false)
		return e, nil
	}
	f, err := os.Open(path)
	if err != nil {
		st.issue(e, err.Error(), true)
		return e, nil
	}
	defer f.Close()
	retain := s.config.MaxDepth == 0 || depth < s.config.MaxDepth
	e.Summarized = !retain
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, readErr := f.ReadDir(256)
		// Stable within batches; avoids retaining a huge directory listing.
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, de := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			childPath := filepath.Join(path, de.Name())
			if (!s.config.ShowHidden && strings.HasPrefix(de.Name(), ".")) || s.isExcluded(childPath, de.Name()) {
				omitted := &models.Entry{Path: childPath}
				st.issue(omitted, "filtered by scan options", false)
				e.Incomplete = true
				continue
			}
			if st.mounts[childPath] {
				child := &models.Entry{Path: childPath, Name: de.Name(), Type: models.EntryTypeDir}
				st.issue(child, "mount omitted before stat (use -cross-filesystems to include)", false)
				e.Incomplete = true
				if retain {
					child.Parent = e
					e.Children = append(e.Children, child)
				}
				continue
			}
			childInfo, err := de.Info()
			var child *models.Entry
			if err != nil {
				child = &models.Entry{Path: childPath, Name: de.Name(), Type: models.EntryTypeOther}
				st.issue(child, err.Error(), true)
			} else {
				child, err = s.visit(ctx, childPath, childInfo, depth+1, st)
				if err != nil {
					return nil, err
				}
			}
			e.LogicalSize += child.LogicalSize
			e.PhysicalSize += child.PhysicalSize
			e.Incomplete = e.Incomplete || child.Incomplete
			if retain {
				child.Parent = e
				e.Children = append(e.Children, child)
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				st.issue(e, readErr.Error(), true)
			}
			break
		}
	}
	models.SortDescendingEntries(e.Children, models.SortByPhysicalSize)
	return e, nil
}

func (st *scanState) keepLargest(e *models.Entry) {
	st.report.LargestFiles = keepLargest(st.report.LargestFiles, e)
}
func keepLargest(entries []*models.Entry, e *models.Entry) []*models.Entry {
	if len(entries) == 20 && e.PhysicalSize <= entries[len(entries)-1].PhysicalSize {
		return entries
	}
	copy := *e
	copy.Parent = nil
	copy.Children = nil
	copy.Report = nil
	entries = append(entries, &copy)
	sort.Slice(entries, func(i, j int) bool { return entries[i].PhysicalSize > entries[j].PhysicalSize })
	if len(entries) > 20 {
		entries = entries[:20]
	}
	return entries
}

func (s *Scanner) isExcluded(path, name string) bool {
	normPath := strings.ReplaceAll(path, `\`, `/`)
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
