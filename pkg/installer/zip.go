package installer

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wimwenigerkind/odoopack/pkg/auth"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/store"
)

type ZipInstaller struct {
	store *store.Store
}

// NewZipInstaller installs through st, or straight into the project when st is
// nil.
func NewZipInstaller(st *store.Store) *ZipInstaller {
	return &ZipInstaller{store: st}
}

func (i *ZipInstaller) Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(targetDir, FormatAddonDir(addonName))

	// Without a shasum there is nothing to look the package up by before
	// downloading it, so it bypasses the store.
	algo, digest, _ := strings.Cut(pkg.Dist.Shasum, ":")
	if i.store == nil || algo != "sha256" {
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		return extractZip(pkg, dest)
	}

	entry, err := i.store.Ensure(strings.ToLower(digest), func(dst string) error {
		return extractZip(pkg, dst)
	})
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return store.Link(entry, dest)
}

// extractZip downloads the package and moves its single root directory to
// dest, which must not exist yet.
func extractZip(pkg lockfile.LockedPackage, dest string) error {
	tmp, err := downloadToTmp(pkg.Dist.URL)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := verifyShasum(tmpPath, pkg.Dist.Shasum); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(filepath.Dir(dest), ".odoopack-unzip-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := unzip(tmpPath, tmpDir, maxZipBytes, maxZipEntries); err != nil {
		return err
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return fmt.Errorf("expected zip to contain a single root directory")
	}

	return os.Rename(filepath.Join(tmpDir, entries[0].Name()), dest)
}

func downloadToTmp(url string) (*os.File, error) {
	tmp, err := os.CreateTemp("", "odoopack-*.zip")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := auth.TokenForURL(url); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("download %s: bad status %s", url, response.Status)
	}

	if _, err = io.Copy(tmp, response.Body); err != nil {
		return nil, err
	}

	return tmp, nil
}

func verifyShasum(path, expected string) error {
	if expected == "" {
		return nil
	}
	algo, want, ok := strings.Cut(expected, ":")
	if !ok || algo != "sha256" {
		return fmt.Errorf("unsupported shasum format %q (want sha256:<hex>)", expected)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("shasum mismatch: got sha256:%s, expected %s — refusing to install a corrupt or tampered zipball", got, expected)
	}
	return nil
}

// Limits on what a zipball may unpack to, so a zip bomb cannot fill the disk.
// They sit far above the largest Odoo addons.
const (
	maxZipBytes   = 1 << 30
	maxZipEntries = 50000
)

// unzip extracts zipPath into destDir. It refuses symlinks and any entry that
// would land outside a single root directory, so that neither the store nor a
// project can end up with a path that points elsewhere.
func unzip(zipPath, destDir string, maxBytes int64, maxEntries int) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	if len(r.File) > maxEntries {
		return fmt.Errorf("zip has %d entries, more than the limit of %d", len(r.File), maxEntries)
	}

	var root string
	var written int64
	for _, f := range r.File {
		name, err := entryPath(f.Name)
		if err != nil {
			return err
		}
		top, _, nested := strings.Cut(name, "/")
		if !nested && !f.Mode().IsDir() {
			return fmt.Errorf("zip entry %q is outside the root directory", f.Name)
		}
		if root == "" {
			root = top
		} else if top != root {
			return fmt.Errorf("zip has more than one root directory: %q and %q", root, top)
		}

		target := filepath.Join(destDir, filepath.FromSlash(name))
		mode := f.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			return fmt.Errorf("zip entry %q is a symlink", f.Name)
		case mode.IsDir():
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case mode.IsRegular():
			n, err := extractFile(f, target, maxBytes-written)
			if err != nil {
				return err
			}
			written += n
			if written > maxBytes {
				return fmt.Errorf("zip unpacks to more than the limit of %d bytes", maxBytes)
			}
		default:
			return fmt.Errorf("zip entry %q has unsupported type %v", f.Name, mode.Type())
		}
	}
	return nil
}

// entryPath returns the cleaned slash separated path of a zip entry name.
func entryPath(name string) (string, error) {
	if strings.Contains(name, "\\") {
		return "", fmt.Errorf("zip entry %q contains a backslash", name)
	}
	if path.IsAbs(name) || (len(name) >= 2 && name[1] == ':') {
		return "", fmt.Errorf("zip entry %q has an absolute path", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("zip entry %q contains a .. component", name)
		}
	}
	// IsLocal also catches what is only special on Windows, such as NUL.
	clean := path.Clean(name)
	if !filepath.IsLocal(filepath.FromSlash(clean)) || clean == "." {
		return "", fmt.Errorf("zip entry %q has an invalid path", name)
	}
	return clean, nil
}

// extractFile writes at most limit+1 bytes of f to target and returns how many
// it wrote. It counts what it actually decompresses, since the sizes in the
// zip header are only claims.
func extractFile(f *zip.File, target string, limit int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return 0, err
	}
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("zip entry %q: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	perm := os.FileMode(0644)
	if f.Mode()&0111 != 0 {
		perm = 0755
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(rc, limit+1))
	if err != nil {
		_ = out.Close()
		return n, fmt.Errorf("zip entry %q: %w", f.Name, err)
	}
	return n, out.Close()
}
