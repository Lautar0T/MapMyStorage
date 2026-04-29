// Package models provides the core data structures for real-disk-map.
package models

import (
	"errors"
	"io/fs"
	"sort"
	"time"
)

// CloudProvider represents the cloud storage provider for a file.
type CloudProvider string

const (
	CloudProviderNone        CloudProvider = ""
	CloudProviderICloud      CloudProvider = "iCloud"
	CloudProviderDropbox     CloudProvider = "Dropbox"
	CloudProviderOneDrive    CloudProvider = "OneDrive"
	CloudProviderGoogleDrive CloudProvider = "GoogleDrive"
	CloudProviderUnknown     CloudProvider = "Unknown"
)

// CloudStatus represents the sync status of a cloud file.
type CloudStatus string

const (
	CloudStatusLocal       CloudStatus = "local"       // Fully downloaded
	CloudStatusSynced      CloudStatus = "synced"      // In sync with cloud
	CloudStatusDownloading CloudStatus = "downloading" // Currently downloading
	CloudStatusOnlineOnly  CloudStatus = "online-only" // Placeholder only
	CloudStatusUnknown     CloudStatus = "unknown"
)

// EntryType represents the type of filesystem entry.
type EntryType string

const (
	EntryTypeFile    EntryType = "file"
	EntryTypeDir     EntryType = "directory"
	EntryTypeSymlink EntryType = "symlink"
	EntryTypeOther   EntryType = "other"
)

// Entry represents a single file or directory with its disk usage information.
type Entry struct {
	Path          string        `json:"path"`
	Name          string        `json:"name"`
	Type          EntryType     `json:"type"`
	LogicalSize   int64         `json:"logical_size"`  // Size reported by stat (st_size)
	PhysicalSize  int64         `json:"physical_size"` // Actual allocated blocks on disk
	ClusterSize   int64         `json:"cluster_size"`  // Filesystem allocation unit size
	IsSymlink     bool          `json:"is_symlink"`
	SymlinkTarget string        `json:"symlink_target,omitempty"`
	IsCloud       bool          `json:"is_cloud"`
	CloudProvider CloudProvider `json:"cloud_provider,omitempty"`
	CloudStatus   CloudStatus   `json:"cloud_status,omitempty"`
	ModTime       time.Time     `json:"mod_time"`
	Permissions   uint32        `json:"permissions"`
	UID           uint32        `json:"uid,omitempty"`
	GID           uint32        `json:"gid,omitempty"`

	// Children is populated for directories
	Children []*Entry `json:"children,omitempty"`

	// Parent reference (not serialized to JSON to avoid cycles)
	Parent *Entry `json:"-"`
}

// TotalLogicalSize returns the total logical size including children.
func (e *Entry) TotalLogicalSize() int64 {
	if e.Type != EntryTypeDir {
		return e.LogicalSize
	}
	var total int64
	for _, child := range e.Children {
		total += child.TotalLogicalSize()
	}
	return total
}

// TotalPhysicalSize returns the total physical size including children.
func (e *Entry) TotalPhysicalSize() int64 {
	if e.Type != EntryTypeDir {
		return e.PhysicalSize
	}
	var total int64
	for _, child := range e.Children {
		total += child.TotalPhysicalSize()
	}
	return total
}

// IsHidden returns true if the entry is a hidden file (starts with .).
func (e *Entry) IsHidden() bool {
	if len(e.Name) == 0 {
		return false
	}
	return e.Name[0] == '.'
}

// SizeRatio returns the ratio of physical to logical size.
// Returns 1.0 if logical size is 0 (to avoid division by zero).
func (e *Entry) SizeRatio() float64 {
	if e.LogicalSize == 0 {
		return 1.0
	}
	return float64(e.PhysicalSize) / float64(e.LogicalSize)
}

// IsSparse returns true if the file is sparse (physical < logical).
func (e *Entry) IsSparse() bool {
	return e.PhysicalSize < e.LogicalSize && !e.IsCloud
}

// IsCompressed returns true if the file is compressed (physical significantly < logical).
func (e *Entry) IsCompressed() bool {
	return e.PhysicalSize < e.LogicalSize/2 && e.LogicalSize > 4096
}

// ScanConfig holds configuration for a scan operation.
type ScanConfig struct {
	RootPath         string   `json:"root_path"`
	FollowSymlinks   bool     `json:"follow_symlinks"`
	ExcludePatterns  []string `json:"exclude_patterns"`
	MaxDepth         int      `json:"max_depth"` // 0 = unlimited
	ShowHidden       bool     `json:"show_hidden"`
	IncludeCloudInfo bool     `json:"include_cloud_info"`
}

// ScanProgress represents the current state of a scan operation.
type ScanProgress struct {
	FilesScanned      int64  `json:"files_scanned"`
	DirsScanned       int64  `json:"dirs_scanned"`
	BytesProcessed    int64  `json:"bytes_processed"`
	CurrentPath       string `json:"current_path"`
	ErrorsEncountered int64  `json:"errors_encountered"`
}

// ScanEvent represents an event emitted during scanning.
type ScanEvent struct {
	Type     ScanEventType `json:"type"`
	Entry    *Entry        `json:"entry,omitempty"`
	Error    error         `json:"-"` // Not serialized
	Progress *ScanProgress `json:"progress,omitempty"`
}

// ScanEventType represents the type of scan event.
type ScanEventType int

const (
	ScanEventStarted ScanEventType = iota
	ScanEventEntry
	ScanEventProgress
	ScanEventError
	ScanEventCompleted
)

// SortField represents the field to sort by.
type SortField int

const (
	SortByPhysicalSize SortField = iota
	SortByLogicalSize
	SortByName
	SortByModTime
	SortBySizeRatio
)

// SortOrder represents the sort direction.
type SortOrder int

const (
	SortAscending SortOrder = iota
	SortDescending
)

// SortOptions holds sorting configuration.
type SortOptions struct {
	Field SortField
	Order SortOrder
}

// ByPhysicalSize implements sort.Interface for []*Entry.
type ByPhysicalSize []*Entry

func (a ByPhysicalSize) Len() int           { return len(a) }
func (a ByPhysicalSize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPhysicalSize) Less(i, j int) bool { return a[i].PhysicalSize > a[j].PhysicalSize }

// ByLogicalSize implements sort.Interface for []*Entry.
type ByLogicalSize []*Entry

func (a ByLogicalSize) Len() int           { return len(a) }
func (a ByLogicalSize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLogicalSize) Less(i, j int) bool { return a[i].LogicalSize > a[j].LogicalSize }

// ByName implements sort.Interface for []*Entry.
type ByName []*Entry

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// ByModTime implements sort.Interface for []*Entry.
type ByModTime []*Entry

func (a ByModTime) Len() int           { return len(a) }
func (a ByModTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByModTime) Less(i, j int) bool { return a[i].ModTime.After(a[j].ModTime) }

// BySizeRatio implements sort.Interface for []*Entry.
type BySizeRatio []*Entry

func (a BySizeRatio) Len() int      { return len(a) }
func (a BySizeRatio) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BySizeRatio) Less(i, j int) bool {
	return a[i].SizeRatio() > a[j].SizeRatio()
}

// WalkEntry walks the entry tree and calls callback for each entry.
func WalkEntry(entry *Entry, callback func(*Entry) error) error {
	if entry == nil {
		return nil
	}

	if err := callback(entry); err != nil {
		if errors.Is(err, fs.SkipDir) {
			return nil
		}
		return err
	}

	if entry.Type != EntryTypeDir {
		return nil
	}

	for _, child := range entry.Children {
		if err := WalkEntry(child, callback); err != nil {
			return err
		}
	}

	return nil
}

// FindEntry finds an entry by absolute path in the tree.
func FindEntry(root *Entry, path string) *Entry {
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

// SortEntries sorts entries according to the provided options.
func SortEntries(entries []*Entry, opts SortOptions) {
	switch opts.Field {
	case SortByPhysicalSize:
		sort.Sort(ByPhysicalSize(entries))
	case SortByLogicalSize:
		sort.Sort(ByLogicalSize(entries))
	case SortByName:
		sort.Sort(ByName(entries))
	case SortByModTime:
		sort.Sort(ByModTime(entries))
	case SortBySizeRatio:
		sort.Sort(BySizeRatio(entries))
	default:
		sort.Sort(ByPhysicalSize(entries))
	}

	if opts.Order == SortAscending {
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}
}

// SortDescendingEntries sorts entries by field in descending order.
func SortDescendingEntries(entries []*Entry, field SortField) {
	SortEntries(entries, SortOptions{
		Field: field,
		Order: SortDescending,
	})
}
