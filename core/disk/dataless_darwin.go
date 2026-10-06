//go:build darwin

package disk

import (
	"os"
	"syscall"
)

func isDataless(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	// SF_DATALESS from Darwin sys/stat.h. stat does not read file contents.
	return ok && st.Flags&0x40000000 != 0
}
