package helper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// WriteJSONFile writes a JSON document with a trailing newline, as editors and
// POSIX tools expect, and atomically: a reader, or an interrupted odoopack,
// sees the old file or the new one, never a truncated mix.
func WriteJSONFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	// After the rename the temporary name is gone, so this only cleans up a
	// failed write.
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp creates the file with 0600; the manifest and lockfile are
	// committed and read by others.
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
