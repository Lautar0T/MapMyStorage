//go:build darwin
// +build darwin

package disk

import "golang.org/x/sys/unix"

// hasXattrDarwin checks if a file has a specific extended attribute on macOS.
func hasXattrDarwin(path, attr string) bool {
	size, err := unix.Getxattr(path, attr, nil)
	return err == nil && size >= 0
}

// getXattrDarwin gets the value of an extended attribute.
func getXattrDarwin(path, attr string) ([]byte, error) {
	size, err := unix.Getxattr(path, attr, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return []byte{}, nil
	}

	buf := make([]byte, size)
	size, err = unix.Getxattr(path, attr, buf)
	if err != nil {
		return nil, err
	}
	return buf[:size], nil
}

// listXattrsDarwin lists all extended attributes of a file.
func listXattrsDarwin(path string) ([]string, error) {
	size, err := unix.Listxattr(path, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return []string{}, nil
	}

	buf := make([]byte, size)
	size, err = unix.Listxattr(path, buf)
	if err != nil {
		return nil, err
	}

	attrs := make([]string, 0)
	start := 0
	for i := 0; i < size; i++ {
		if buf[i] == 0 {
			attrs = append(attrs, string(buf[start:i]))
			start = i + 1
		}
	}

	return attrs, nil
}
