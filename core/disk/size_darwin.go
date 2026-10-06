//go:build darwin
// +build darwin

package disk

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// GetPhysicalSize returns actual allocated bytes on macOS using st_blocks * 512.
func GetPhysicalSize(path string, info os.FileInfo) (int64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("stat_t unavailable for %s", path)
	}

	physicalSize := stat.Blocks * 512
	// Zero blocks is valid for sparse files and cloud placeholders.

	return physicalSize, nil
}

// GetLogicalSize returns the logical size (st_size).
func GetLogicalSize(info os.FileInfo) int64 {
	return info.Size()
}

// GetClusterSize returns filesystem block size for the path.
func GetClusterSize(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to statfs: %w", err)
	}
	return int64(stat.Bsize), nil
}

// IsCompressed checks if the file has decmpfs metadata.
func IsCompressed(path string) (bool, error) {
	size, err := getXattrSize(path, "com.apple.decmpfs")
	if err != nil {
		return false, nil
	}
	return size > 0, nil
}

func getXattrSize(path string, attr string) (int, error) {
	return unix.Getxattr(path, attr, nil)
}

// GetExtendedAttributes returns all xattr names for a path.
func GetExtendedAttributes(path string) ([]string, error) {
	size, err := unix.Listxattr(path, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return []string{}, nil
	}

	buf := make([]byte, size)
	size, err = unix.Listxattr(path, buf)
	if err != nil {
		return nil, err
	}

	attrs := make([]string, 0)
	start := 0
	for i := 0; i < size; i++ {
		if buf[i] == 0 {
			attrs = append(attrs, string(buf[start:i]))
			start = i + 1
		}
	}

	return attrs, nil
}

// HasExtendedAttribute checks whether attr exists on path.
func HasExtendedAttribute(path string, attr string) bool {
	size, err := unix.Getxattr(path, attr, nil)
	return err == nil && size >= 0
}

// GetAllocationSize returns allocation size using direct stat.
func GetAllocationSize(path string) (int64, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0, err
	}
	return stat.Blocks * 512, nil
}

// FileSizeInfo holds size information for a file.
type FileSizeInfo struct {
	LogicalSize  int64
	PhysicalSize int64
	ClusterSize  int64
}

// GetFileInfo returns logical + physical metadata for a file.
func GetFileInfo(path string) (*FileSizeInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	logicalSize := info.Size()
	physicalSize, err := GetPhysicalSize(path, info)
	if err != nil {
		physicalSize = 0
	}

	clusterSize, _ := GetClusterSize(path)

	return &FileSizeInfo{
		LogicalSize:  logicalSize,
		PhysicalSize: physicalSize,
		ClusterSize:  clusterSize,
	}, nil
}

// IsSparseFile returns true if allocated blocks are much smaller than logical bytes.
func IsSparseFile(path string, info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	expectedBlocks := (stat.Size + 511) / 512
	return stat.Blocks < expectedBlocks/2 && stat.Size > 4096
}
