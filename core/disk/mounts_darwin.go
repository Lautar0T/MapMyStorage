//go:build darwin

package disk

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Read the cached mount table before statting children. Even lstat at a network
// mount can block, so a device-ID comparison after stat is too late.
func excludedMounts(root string, wholeDisk bool) (map[string]bool, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	mounts := make([]unix.Statfs_t, n+16)
	n, err = unix.Getfsstat(mounts, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	if n > len(mounts) {
		n = len(mounts)
	}
	excluded := make(map[string]bool)
	for _, m := range mounts[:n] {
		path := strings.TrimRight(string(m.Mntonname[:]), "\x00")
		path = filepath.Clean(path)
		if path == "/" || path == root || strings.HasPrefix(root, path+"/") {
			continue
		}
		if wholeDisk && (path == "/System/Volumes/Data" || path == "/System/Volumes/VM") {
			continue
		}
		excluded[path] = true
		// The same mounted location can be reached through the Data firmlink view.
		if !strings.HasPrefix(path, "/System/Volumes/") {
			excluded["/System/Volumes/Data"+path] = true
		}
	}
	return excluded, nil
}

// diskutil info accepts a device or volume mount point, not arbitrary subfolders.
func diagnosticVolume(path string) (string, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return "", err
	}
	return strings.TrimRight(string(st.Mntonname[:]), "\x00"), nil
}
