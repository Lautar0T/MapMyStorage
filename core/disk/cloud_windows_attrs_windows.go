//go:build windows

package disk

import (
	"os"

	"github.com/lautar0t/MapMyStorage/core/models"
	"golang.org/x/sys/windows"
)

const (
	fileAttributePinned             = 0x00080000
	fileAttributeUnpinned           = 0x00100000
	fileAttributeRecallOnOpen       = 0x00040000
	fileAttributeRecallOnDataAccess = 0x00400000
)

func detectWindowsCloudState(path string, info os.FileInfo) (models.CloudStatus, bool) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return models.CloudStatusUnknown, false
	}

	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return models.CloudStatusUnknown, false
	}

	if attrs&fileAttributeRecallOnDataAccess != 0 || attrs&fileAttributeRecallOnOpen != 0 || attrs&windows.FILE_ATTRIBUTE_OFFLINE != 0 {
		return models.CloudStatusOnlineOnly, true
	}
	if attrs&fileAttributePinned != 0 {
		return models.CloudStatusLocal, true
	}

	// Unpinned is an eviction policy, not evidence that data is absent.
	return models.CloudStatusUnknown, false
}
