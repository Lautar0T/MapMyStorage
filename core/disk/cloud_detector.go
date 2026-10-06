// Package disk provides filesystem scanning and disk usage calculation.
package disk

import (
	"os"
	"runtime"
	"strings"

	"github.com/lautar0t/MapMyStorage/core/models"
)

// CloudDetector detects cloud storage files and their sync status.
type CloudDetector struct {
	// Platform-specific implementation details
}

// NewCloudDetector creates a new cloud detector.
func NewCloudDetector() *CloudDetector {
	return &CloudDetector{}
}

// DetectCloudStatus determines if a file is managed by a cloud provider and its sync status.
func (cd *CloudDetector) DetectCloudStatus(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	switch runtime.GOOS {
	case "darwin":
		return cd.detectMacOS(path, info)
	case "windows":
		return cd.detectWindows(path, info)
	default:
		return cd.detectGeneric(path, info)
	}
}

// detectMacOS uses File Provider paths and Darwin's dataless flag. A low
// allocation ratio alone cannot distinguish a sparse file from a placeholder.
func (cd *CloudDetector) detectMacOS(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	provider := GetCloudProviderFromPath(path)
	if isDataless(info) {
		if provider == models.CloudProviderNone {
			provider = models.CloudProviderUnknown
		}
		return provider, models.CloudStatusOnlineOnly, nil
	}
	if provider != models.CloudProviderNone {
		return provider, cd.inferStatusFromSize(path, info), nil
	}
	return models.CloudProviderNone, models.CloudStatusLocal, nil
}

// detectWindows combines provider path hints with explicit placeholder attributes.
func (cd *CloudDetector) detectWindows(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	provider := GetCloudProviderFromPath(path)
	status, signal := detectWindowsCloudState(path, info)
	if signal {
		if provider == models.CloudProviderNone {
			provider = models.CloudProviderUnknown
		}
		return provider, status, nil
	}
	if provider != models.CloudProviderNone {
		return provider, cd.inferStatusFromSize(path, info), nil
	}
	return models.CloudProviderNone, models.CloudStatusLocal, nil
}

// detectGeneric identifies provider paths but does not claim a sync state.
func (cd *CloudDetector) detectGeneric(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	provider := GetCloudProviderFromPath(path)
	if provider != models.CloudProviderNone {
		return provider, cd.inferStatusFromSize(path, info), nil
	}
	return models.CloudProviderNone, models.CloudStatusLocal, nil
}

// inferStatusFromSize infers cloud status from physical vs logical size.
func (cd *CloudDetector) inferStatusFromSize(path string, info os.FileInfo) models.CloudStatus {
	if info.IsDir() {
		return models.CloudStatusUnknown
	}

	logicalSize := info.Size()
	if logicalSize == 0 {
		return models.CloudStatusLocal
	}

	// Get physical size
	physicalSize, err := GetPhysicalSize(path, info)
	if err != nil {
		return models.CloudStatusUnknown
	}

	// Allocation supports a local hint, not a guarantee of synchronization.
	if physicalSize >= logicalSize*95/100 {
		return models.CloudStatusLocal
	}

	// Otherwise it's partially synced or compressed
	return models.CloudStatusUnknown
}

// hasXattrMacOS checks if a file has a specific extended attribute on macOS.
// This is a stub that will be implemented with build tags.
func (cd *CloudDetector) hasXattrMacOS(path, attr string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	return hasXattrDarwin(path, attr)
}

// IsCloudPlaceholder checks if a file is a cloud placeholder (not downloaded).
func (cd *CloudDetector) IsCloudPlaceholder(path string, info os.FileInfo) bool {
	provider, status, _ := cd.DetectCloudStatus(path, info)
	if provider == models.CloudProviderNone {
		return false
	}
	return status == models.CloudStatusOnlineOnly
}

// GetCloudProviderFromPath attempts to identify cloud provider from the path.
func GetCloudProviderFromPath(path string) models.CloudProvider {
	parts := strings.Split(strings.ReplaceAll(strings.ToLower(path), `\`, "/"), "/")
	for i, part := range parts {
		// Client databases/caches are local application data, not synced files.
		// Stop only before reaching a provider root (a user may sync a folder named Caches).
		if part == "application support" || part == "caches" || part == "containers" || part == "group containers" {
			return models.CloudProviderNone
		}
		// Match provider directory components, never arbitrary substrings in filenames.
		if i == len(parts)-1 && strings.Contains(part, ".") {
			continue
		}
		switch {
		case part == "dropbox" || strings.HasPrefix(part, "dropbox (") || strings.HasPrefix(part, "dropbox-"):
			return models.CloudProviderDropbox
		case part == "onedrive" || strings.HasPrefix(part, "onedrive-") || strings.HasPrefix(part, "onedrive - "):
			return models.CloudProviderOneDrive
		case part == "google drive" || strings.HasPrefix(part, "googledrive-") || part == "google-drive":
			return models.CloudProviderGoogleDrive
		case part == "com~apple~clouddocs" || part == "icloud drive":
			return models.CloudProviderICloud
		}
	}
	return models.CloudProviderNone
}
