package store

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Store keeps every extracted package once, keyed by the sha256 of its
// archive, and links it into each project that installs it.
type Store struct {
	dir string

	// Ensure takes a shared hold on the lock file and Link gives it back.
	// Concurrent installs through one Store share a single lock.
	mu    sync.Mutex
	holds int
	lock  *os.File
}

// Installs lock the file shared and prune exclusively, so prune cannot remove
// an entry between an install's Ensure and its Link, and installs do not
// block each other.
const lockName = ".lock"

func New(dir string) *Store {
	return &Store{dir: dir}
}

// Open returns the store under the configured cache directory.
func Open() (*Store, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	return New(filepath.Join(dir, "store")), nil
}

// Default returns the store installs go through, or nil when caching is turned
// off.
func Default() (*Store, error) {
	if viper.GetBool("no_cache") {
		return nil, nil
	}
	return Open()
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
//
// A successful Ensure keeps prune away from the store until Link has linked
// the entry, so it must be followed by Link.
func (s *Store) Ensure(digest string, fill func(dst string) error) (entry string, err error) {
	// The digest comes from the lockfile and becomes a path, so it must not
	// be able to point outside the store.
	if !digestPattern.MatchString(digest) {
		return "", fmt.Errorf("invalid store digest %q", digest)
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return "", err
	}
	if err := s.hold(); err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			s.release()
		}
	}()

	entry = filepath.Join(s.dir, digest)
	if _, err := os.Stat(entry); err == nil {
		return entry, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
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

type PruneResult struct {
	Entries int
	Bytes   int64
}

// A staging directory younger than this may belong to an install that is
// still running.
const staleStagingAge = time.Hour

// Prune removes every entry no project links to any more, or every entry at
// all. An entry is in use while one of its files has a second hard link, so
// the store needs no record of the projects that use it. Prune waits for
// running installs to link their entries, and installs wait for Prune.
func (s *Store) Prune(all bool) (PruneResult, error) {
	var res PruneResult
	if _, err := os.Stat(s.dir); errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}
	lock, err := openLock(s.dir)
	if err != nil {
		return res, err
	}
	defer func() { _ = lock.Close() }()
	if err := lockFile(lock, true); err != nil {
		return res, err
	}
	defer func() { _ = unlockFile(lock) }()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return res, err
	}

	for _, e := range entries {
		path := filepath.Join(s.dir, e.Name())
		staging := strings.HasPrefix(e.Name(), ".tmp-")
		// Leave alone whatever the store did not create.
		if !staging && !digestPattern.MatchString(e.Name()) {
			continue
		}

		if staging && !all {
			info, err := e.Info()
			if err != nil {
				return res, err
			}
			if time.Since(info.ModTime()) < staleStagingAge {
				continue
			}
		}

		size, used, err := usage(path)
		if err != nil {
			return res, err
		}
		if used && !all {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return res, err
		}
		res.Bytes += size
		if !staging {
			res.Entries++
		}
	}
	return res, nil
}

func usage(root string) (size int64, used bool, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		links, err := linkCount(path, info)
		if err != nil {
			return err
		}
		if links > 1 {
			used = true
		}
		return nil
	})
	return size, used, err
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

// Link recreates the entry Ensure returned at dst with every file hard linked
// to the store, and ends the hold Ensure took. A file that cannot be linked,
// for example because dst is on another volume, is copied instead.
func (s *Store) Link(entry, dst string) error {
	defer s.release()
	return link(entry, dst)
}

func link(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.Link(path, target); err == nil {
			return nil
		}
		return copyFile(path, target)
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

func openLock(dir string) (*os.File, error) {
	return os.OpenFile(filepath.Join(dir, lockName), os.O_RDWR|os.O_CREATE, 0644)
}

func (s *Store) hold() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.holds == 0 {
		lock, err := openLock(s.dir)
		if err != nil {
			return err
		}
		if err := lockFile(lock, false); err != nil {
			_ = lock.Close()
			return err
		}
		s.lock = lock
	}
	s.holds++
	return nil
}

func (s *Store) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holds--
	if s.holds == 0 {
		_ = unlockFile(s.lock)
		_ = s.lock.Close()
		s.lock = nil
	}
}
