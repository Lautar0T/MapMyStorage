//go:build !darwin

package disk

import "os"

func isDataless(info os.FileInfo) bool { return false }
