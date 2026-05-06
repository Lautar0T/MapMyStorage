//go:build windows
// +build windows

package disk

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL                = windows.NewLazySystemDLL("kernel32.dll")
	procGetCompressedFileSizeW = kernel32DLL.NewProc("GetCompressedFileSizeW")
	procGetDiskFreeSpaceW      = kernel32DLL.NewProc("GetDiskFreeSpaceW")
)

// GetPhysicalSize returns allocated bytes on disk using GetCompressedFileSizeW.
func GetPhysicalSize(path string, info os.FileInfo) (int64, error) {
	size, err := getCompressedFileSize(path)
	if err != nil {
		return 0, err
	}

	clusterSize, cErr := GetClusterSize(path)
	if cErr == nil && clusterSize > 0 {
		clusters := (int64(size) + clusterSize - 1) / clusterSize
		return clusters * clusterSize, nil
	}

	return int64(size), nil
}

func getCompressedFileSize(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("failed to convert path: %w", err)
	}

	var high uint32
	r1, _, callErr := procGetCompressedFileSizeW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&high)),
	)
	low := uint32(r1)
	if low == 0xFFFFFFFF {
		if errno, ok := callErr.(windows.Errno); ok && errno != 0 {
			return 0, errno
		}
	}

	return uint64(high)<<32 | uint64(low), nil
}

// GetLogicalSize returns logical file size.
func GetLogicalSize(info os.FileInfo) int64 {
	return info.Size()
}

// GetClusterSize returns allocation unit size of the filesystem.
func GetClusterSize(path string) (int64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var volumePath [windows.MAX_PATH]uint16
	if err := windows.GetVolumePathName(pathPtr, &volumePath[0], windows.MAX_PATH); err != nil {
		return 0, fmt.Errorf("failed to get volume path: %w", err)
	}

	var sectorsPerCluster, bytesPerSector, freeClusters, totalClusters uint32
	r1, _, callErr := procGetDiskFreeSpaceW.Call(
		uintptr(unsafe.Pointer(&volumePath[0])),
		uintptr(unsafe.Pointer(&sectorsPerCluster)),
		uintptr(unsafe.Pointer(&bytesPerSector)),
		uintptr(unsafe.Pointer(&freeClusters)),
		uintptr(unsafe.Pointer(&totalClusters)),
	)
	if r1 == 0 {
		if errno, ok := callErr.(windows.Errno); ok {
			return 0, errno
		}
		return 0, fmt.Errorf("GetDiskFreeSpaceW failed")
	}

	return int64(sectorsPerCluster) * int64(bytesPerSector), nil
}

// IsSparseFile checks sparse attribute.
func IsSparseFile(path string) (bool, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}

	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return false, err
	}

	return attrs&windows.FILE_ATTRIBUTE_SPARSE_FILE != 0, nil
}

// IsCompressedFile checks compressed attribute.
func IsCompressedFile(path string) (bool, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}

	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return false, err
	}

	return attrs&windows.FILE_ATTRIBUTE_COMPRESSED != 0, nil
}

// FileSizeInfo holds size information for a file.
type FileSizeInfo struct {
	LogicalSize  int64
	PhysicalSize int64
	ClusterSize  int64
}

// GetFileInfo returns detailed size info for a path.
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

// IsReparsePoint checks if path is a reparse point.
func IsReparsePoint(path string) (bool, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}

	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return false, err
	}

	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

// GetReparseTag returns the reparse tag for a reparse point.
func GetReparseTag(path string) (uint32, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(handle)

	const FSCTL_GET_REPARSE_POINT = 0x000900a8

	var reparseData [windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE]byte
	var bytesReturned uint32

	err = windows.DeviceIoControl(
		handle,
		FSCTL_GET_REPARSE_POINT,
		nil,
		0,
		&reparseData[0],
		uint32(len(reparseData)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, err
	}

	return *(*uint32)(unsafe.Pointer(&reparseData[0])), nil
}
