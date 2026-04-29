//go:build !darwin
// +build !darwin

package disk

func hasXattrDarwin(path, attr string) bool {
	return false
}
