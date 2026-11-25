package helper

import (
	"errors"
	"fmt"
	"os"
)

func FileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	if info.IsDir() {
		return false, nil
	}

	return true, nil
}

// CheckFormatVersion returns the format version of the file at path: a file
// without the field is version 1, and a version above current is refused, so
// that an older odoopack does not misread a file a newer one wrote.
func CheckFormatVersion(path string, got, current int) (int, error) {
	switch {
	case got == 0:
		return 1, nil
	case got < 0:
		return 0, fmt.Errorf("%s: invalid format_version %d", path, got)
	case got > current:
		return 0, fmt.Errorf("%s has format_version %d, but this odoopack only reads up to format_version %d; upgrade odoopack", path, got, current)
	}
	return got, nil
}
