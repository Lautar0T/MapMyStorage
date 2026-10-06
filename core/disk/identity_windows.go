//go:build windows

package disk

import "os"

// Win32FileAttributeData has no file ID. Do not guess an inode from it.
func fileIdentity(info os.FileInfo) string   { return "" }
func deviceIdentity(info os.FileInfo) string { return "" }

func hasMultipleLinks(info os.FileInfo) bool { return false }
