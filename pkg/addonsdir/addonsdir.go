// Package addonsdir guards the addons_path directory, which install clears and
// rebuilds, so that odoopack only ever deletes what it created itself.
package addonsdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	MarkerName    = ".odoopack"
	markerContent = "This directory is managed by odoopack: install deletes and rebuilds it.\n"
)

// Check reports whether path is usable as addons_path for the project in
// projectDir: relative, and resolving to a directory strictly inside the
// project, also after following symlinks of the parts that already exist.
func Check(projectDir, path string) error {
	if path == "" {
		return errors.New("addons_path is not set")
	}
	if !filepath.IsLocal(path) {
		return fmt.Errorf("addons_path %q must be a relative path inside the project", path)
	}
	if filepath.Clean(path) == "." {
		return fmt.Errorf("addons_path %q must not be the project directory itself", path)
	}

	root, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		return err
	}
	resolved, err := resolve(filepath.Join(root, path))
	if err != nil {
		return fmt.Errorf("addons_path %q: %w", path, err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("addons_path %q resolves to %s, which is not inside the project", path, resolved)
	}
	return nil
}

// resolve follows the symlinks of the longest existing prefix of path and
// appends the parts that do not exist yet.
func resolve(path string) (string, error) {
	rest := ""
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		// A dangling symlink does not exist for EvalSymlinks, but creating
		// the directory would follow it.
		if _, lerr := os.Lstat(path); lerr == nil {
			return "", fmt.Errorf("%s is a symlink to a missing target", path)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}

// Claim creates dir with the marker, or adds the marker to an existing dir
// that is empty or adopted (see checkOwned). Other content is refused.
// locked holds the addon directory names of the packages in the lockfile.
func Claim(dir string, locked map[string]bool) error {
	if err := checkOwned(dir, locked); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return writeMarker(dir)
}

// Clear deletes dir and claims it again empty.
func Clear(dir string, locked map[string]bool) error {
	if err := checkOwned(dir, locked); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return Claim(dir, nil)
}

// RemoveAddon deletes the directory addonDir inside dir.
func RemoveAddon(dir, addonDir string, locked map[string]bool) error {
	if !filepath.IsLocal(addonDir) || filepath.Base(addonDir) != addonDir || addonDir == "." {
		return fmt.Errorf("addon directory %q is not a single name inside addons_path", addonDir)
	}
	if err := checkOwned(dir, locked); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, addonDir))
}

// checkOwned accepts a missing or empty dir, or one with the marker. A dir
// without the marker that holds nothing but directories named in locked was
// filled by an odoopack version before the marker existed: it is adopted and
// gets the marker.
func checkOwned(dir string, locked map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || err == nil && len(entries) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	if info, err := os.Lstat(filepath.Join(dir, MarkerName)); err == nil && info.Mode().IsRegular() {
		return nil
	}
	for _, e := range entries {
		// ReadDir does not follow symlinks, so a link named like a locked
		// addon is not a directory here and is refused.
		if !e.IsDir() || !locked[e.Name()] {
			return fmt.Errorf("addons_path %s is not empty and was not created by odoopack (no %s file, and %s is not an installed addon); empty it or choose another addons_path", dir, MarkerName, e.Name())
		}
	}
	return writeMarker(dir)
}

func writeMarker(dir string) error {
	return os.WriteFile(filepath.Join(dir, MarkerName), []byte(markerContent), 0644)
}
