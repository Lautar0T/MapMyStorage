//go:build !darwin

package disk

func excludedMounts(root string, wholeDisk bool) (map[string]bool, error) { return nil, nil }
