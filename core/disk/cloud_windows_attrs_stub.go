//go:build !windows

package disk

import (
	"os"

	"github.com/lautaro/real-disk-map/core/models"
)

func detectWindowsCloudState(path string, info os.FileInfo) (models.CloudStatus, bool) {
	return models.CloudStatusUnknown, false
}
