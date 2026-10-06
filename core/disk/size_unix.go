//go:build unix && !darwin
// +build unix,!darwin

package disk

import (
	"fmt"
	"os"
	"syscall"
)

// GetPhysicalSize returns the actual disk space allocated for the file on Unix/Linux.
// This uses st_blocks * 512 to get the physical size.
func GetPhysicalSize(path string, info os.FileInfo) (int64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("allocated blocks unavailable for %s", path)
	}

	// st_blocks is the number of 512-byte blocks allocated
	physicalSize := stat.Blocks * 512

	return physicalSize, nil
}

// GetLogicalSize returns the logical file size.
func GetLogicalSize(info os.FileInfo) int64 {
	return info.Size()
}

// GetClusterSize returns the filesystem block size.
func GetClusterSize(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to statfs: %w", err)
	}
	return int64(stat.Bsize), nil
}

// IsSparseFile checks if a file is sparse.
func IsSparseFile(path string, info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	expectedBlocks := (stat.Size + 511) / 512
	return stat.Blocks < expectedBlocks/2 && stat.Size > 4096
}

// GetFileInfo returns detailed file info.
func GetFileInfo(path string) (*FileSizeInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	logicalSize := info.Size()
	physicalSize, err := GetPhysicalSize(path, info)
	if err != nil {
		physicalSize = logicalSize
	}

	clusterSize, _ := GetClusterSize(path)

	return &FileSizeInfo{
		LogicalSize:  logicalSize,
		PhysicalSize: physicalSize,
		ClusterSize:  clusterSize,
	}, nil
}

// FileSizeInfo holds size information for a file.
type FileSizeInfo struct {
	LogicalSize  int64
	PhysicalSize int64
	ClusterSize  int64
}

// IsCompressed always returns false on generic Unix (no native compression detection).
func IsCompressed(path string) (bool, error) {
	return false, nil
}
