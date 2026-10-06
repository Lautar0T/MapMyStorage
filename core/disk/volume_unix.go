//go:build !windows

package disk

import (
	"github.com/lautar0t/MapMyStorage/core/models"
	"syscall"
)

func VolumeUsage(path string) (*models.VolumeUsage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	return &models.VolumeUsage{Path: path, Capacity: int64(st.Blocks) * int64(st.Bsize), Free: int64(st.Bfree) * int64(st.Bsize), Available: int64(st.Bavail) * int64(st.Bsize)}, nil
}
