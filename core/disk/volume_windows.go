//go:build windows

package disk

import (
	"github.com/lautar0t/MapMyStorage/core/models"
	"golang.org/x/sys/windows"
)

func VolumeUsage(path string) (*models.VolumeUsage, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &available, &total, &free); err != nil {
		return nil, err
	}
	return &models.VolumeUsage{Path: path, Capacity: int64(total), Free: int64(free), Available: int64(available)}, nil
}
