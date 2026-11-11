package store

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/viper"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Store keeps every extracted package once, keyed by the sha256 of its
// archive, and links it into each project that installs it.
type Store struct {
	dir string
}

func New(dir string) *Store {
	return &Store{dir: dir}
}

// Default returns the store under the configured cache directory, or nil when
// caching is turned off.
func Default() (*Store, error) {
	if viper.GetBool("no_cache") {
		return nil, nil
	}
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	return New(filepath.Join(dir, "store")), nil
}

// CacheDir follows XDG rather than os.UserCacheDir, which would put the cache
// into ~/Library/Caches on macOS.
func CacheDir() (string, error) {
	if dir := viper.GetString("cache_dir"); dir != "" {
		return dir, nil
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "odoopack"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "odoopack"), nil
}

// Ensure returns the entry for digest, calling fill to create it when it is
// missing. fill receives a path that does not exist yet and must create it.
func (s *Store) Ensure(digest string, fill func(dst string) error) (string, error) {
	// The digest comes from the lockfile and becomes a path, so it must not
	// be able to point outside the store.
	if !digestPattern.MatchString(digest) {
		return "", fmt.Errorf("invalid store digest %q", digest)
	}

	entry := filepath.Join(s.dir, digest)
	if _, err := os.Stat(entry); err == nil {
		return entry, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(s.dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	staged := filepath.Join(tmp, digest)
	if err := fill(staged); err != nil {
		return "", err
	}
	if err := makeReadOnly(staged); err != nil {
		return "", err
	}

	// Another process may have added the same entry in the meantime; its
	// content is identical, so losing the race is fine.
	if err := os.Rename(staged, entry); err != nil {
		if _, statErr := os.Stat(entry); statErr != nil {
			return "", err
		}
	}
	return entry, nil
}

// Files are hard linked into projects, so an edit in a project would change
// the store and every other project; read-only files make such an edit fail
// instead. Directories stay writable so entries can be removed again.
func makeReadOnly(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.Chmod(path, info.Mode().Perm()&^0222)
	})
}

// Link recreates src at dst with every file hard linked to the store. A file
// that cannot be linked, for example because dst is on another volume, is
// copied instead.
func Link(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			if err := os.Link(path, target); err == nil {
				return nil
			}
			return copyFile(path, target)
		}
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	// A copy is not shared with the store, so it can stay writable.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()|0200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
