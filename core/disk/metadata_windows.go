//go:build windows

package disk

import "os"

func extractOwnership(_ os.FileInfo) (uint32, uint32) {
	return 0, 0
}
