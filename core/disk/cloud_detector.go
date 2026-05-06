// Package disk provides filesystem scanning and disk usage calculation.
package disk

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lautaro/real-disk-map/core/models"
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

// detectMacOS detects cloud files on macOS using extended attributes.
func (cd *CloudDetector) detectMacOS(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	// Check for iCloud
	if cd.hasXattrMacOS(path, "com.apple.icloud.itemName") {
		// Check availability
		status := models.CloudStatusLocal
		if cd.hasXattrMacOS(path, "com.apple.availability") {
			// This is a placeholder, we need to check the actual attribute value
			// For now, assume online-only if the physical size is very small
			if info.Size() > 0 {
				physicalSize, _ := GetPhysicalSize(path, info)
				if physicalSize < 1024 && info.Size() > 1024 {
					status = models.CloudStatusOnlineOnly
				}
			}
		}
		return models.CloudProviderICloud, status, nil
	}

	// Check for Dropbox
	if cd.hasXattrMacOS(path, "com.dropbox.attributes") ||
		cd.hasXattrMacOS(path, "com.dropbox.attrs") {
		return models.CloudProviderDropbox, models.CloudStatusUnknown, nil
	}

	// Check for OneDrive
	if cd.hasXattrMacOS(path, "com.microsoft.OneDrive.ExtendedProperties") ||
		cd.hasXattrMacOS(path, "com.microsoft.OneDrive.StreamingAttributes") {
		return models.CloudProviderOneDrive, models.CloudStatusUnknown, nil
	}

	// Check for Google Drive (if available)
	if cd.hasXattrMacOS(path, "com.google.drivefs") {
		return models.CloudProviderGoogleDrive, models.CloudStatusUnknown, nil
	}

	return models.CloudProviderNone, models.CloudStatusLocal, nil
}

// detectWindows detects cloud files on Windows using file attributes.
func (cd *CloudDetector) detectWindows(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	lowerPath := strings.ToLower(path)
	statusFromAttrs, hasAttrSignal := detectWindowsCloudState(path, info)

	// Check if in OneDrive folder
	if strings.Contains(lowerPath, "\\onedrive") || strings.Contains(lowerPath, "/onedrive") {
		if hasAttrSignal {
			return models.CloudProviderOneDrive, statusFromAttrs, nil
		}
		return models.CloudProviderOneDrive, cd.inferStatusFromSize(path, info), nil
	}

	// Check if in Dropbox folder
	if strings.Contains(lowerPath, "\\dropbox") || strings.Contains(lowerPath, "/dropbox") {
		return models.CloudProviderDropbox, cd.inferStatusFromSize(path, info), nil
	}

	// Check if in Google Drive folder
	if strings.Contains(lowerPath, "\\google drive") || strings.Contains(lowerPath, "/google drive") ||
		strings.Contains(lowerPath, "\\my drive") || strings.Contains(lowerPath, "/my drive") {
		return models.CloudProviderGoogleDrive, cd.inferStatusFromSize(path, info), nil
	}

	// Check if in iCloud Drive folder (rare on Windows but possible)
	if strings.Contains(lowerPath, "\\icloud drive") || strings.Contains(lowerPath, "/icloud drive") {
		return models.CloudProviderICloud, cd.inferStatusFromSize(path, info), nil
	}

	if hasAttrSignal {
		// Attributes indicate a cloud placeholder/local state even if the folder
		// name does not include provider branding.
		return models.CloudProviderUnknown, statusFromAttrs, nil
	}

	return models.CloudProviderNone, models.CloudStatusLocal, nil
}

// detectGeneric uses heuristics for other platforms.
func (cd *CloudDetector) detectGeneric(path string, info os.FileInfo) (models.CloudProvider, models.CloudStatus, error) {
	// Use path-based detection
	lowerPath := strings.ToLower(path)

	if strings.Contains(lowerPath, "/dropbox/") || strings.Contains(lowerPath, "\\dropbox\\") {
		return models.CloudProviderDropbox, cd.inferStatusFromSize(path, info), nil
	}

	if strings.Contains(lowerPath, "/onedrive/") || strings.Contains(lowerPath, "\\onedrive\\") {
		return models.CloudProviderOneDrive, cd.inferStatusFromSize(path, info), nil
	}

	if strings.Contains(lowerPath, "/google drive/") || strings.Contains(lowerPath, "\\google drive\\") {
		return models.CloudProviderGoogleDrive, cd.inferStatusFromSize(path, info), nil
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

	// If physical size is less than 1% of logical size, it's likely online-only
	if physicalSize < logicalSize/100 {
		return models.CloudStatusOnlineOnly
	}

	// If physical size is close to logical size, it's local
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
	lowerPath := strings.ToLower(path)
	base := filepath.Base(lowerPath)

	if strings.Contains(base, "icloud") {
		return models.CloudProviderICloud
	}
	if strings.Contains(base, "dropbox") {
		return models.CloudProviderDropbox
	}
	if strings.Contains(base, "onedrive") {
		return models.CloudProviderOneDrive
	}
	if strings.Contains(base, "google drive") || strings.Contains(base, "google-drive") {
		return models.CloudProviderGoogleDrive
	}

	// Check parent directories
	dir := filepath.Dir(lowerPath)
	for dir != "/" && dir != "." {
		base = filepath.Base(dir)
		if strings.Contains(base, "icloud") {
			return models.CloudProviderICloud
		}
		if strings.Contains(base, "dropbox") {
			return models.CloudProviderDropbox
		}
		if strings.Contains(base, "onedrive") {
			return models.CloudProviderOneDrive
		}
		if strings.Contains(base, "google drive") || strings.Contains(base, "google-drive") {
			return models.CloudProviderGoogleDrive
		}
		dir = filepath.Dir(dir)
	}

	return models.CloudProviderNone
}
