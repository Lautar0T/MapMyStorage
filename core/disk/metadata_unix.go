//go:build !windows

package disk

import (
	"os"
	"syscall"
)

func extractOwnership(info os.FileInfo) (uint32, uint32) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return stat.Uid, stat.Gid
}
