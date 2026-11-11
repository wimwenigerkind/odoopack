package installer

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/store"
)

func serveAddonZip(t *testing.T) (*httptest.Server, string, *atomic.Int32) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("health_check-1.0/__manifest__.py")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	body := buf.Bytes()

	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, fmt.Sprintf("sha256:%x", sha256.Sum256(body)), &downloads
}

func TestZipInstallReusesStore(t *testing.T) {
	srv, shasum, downloads := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Shasum: shasum}}
	inst := NewZipInstaller(store.New(t.TempDir()))

	first := t.TempDir()
	second := t.TempDir()
	for _, target := range []string{first, second} {
		if err := inst.Install(target, "acme/health-check", pkg); err != nil {
			t.Fatal(err)
		}
	}
	if n := downloads.Load(); n != 1 {
		t.Fatalf("downloaded %d times, want 1", n)
	}

	a, err := os.Stat(filepath.Join(first, "acme_health_check", "__manifest__.py"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(filepath.Join(second, "acme_health_check", "__manifest__.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(a, b) {
		t.Fatal("projects do not share the store file")
	}
}

func TestZipInstallWithoutStore(t *testing.T) {
	srv, shasum, downloads := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Shasum: shasum}}
	inst := NewZipInstaller(nil)

	target := t.TempDir()
	for range 2 {
		if err := inst.Install(target, "acme/health-check", pkg); err != nil {
			t.Fatal(err)
		}
	}
	if n := downloads.Load(); n != 2 {
		t.Fatalf("downloaded %d times, want 2", n)
	}

	info, err := os.Stat(filepath.Join(target, "acme_health_check", "__manifest__.py"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0200 == 0 {
		t.Fatalf("file mode %v is not writable without the store", info.Mode().Perm())
	}
}

func TestZipInstallRejectsShasumMismatch(t *testing.T) {
	srv, _, _ := serveAddonZip(t)
	wrong := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("other")))
	pkg := lockfile.LockedPackage{Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Shasum: wrong}}
	storeDir := t.TempDir()
	inst := NewZipInstaller(store.New(storeDir))

	if err := inst.Install(t.TempDir(), "acme/health-check", pkg); err == nil {
		t.Fatal("expected a shasum mismatch")
	}
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("store keeps %d entries after a mismatch", len(entries))
	}
}

func TestZipInstallRejectsMalformedShasum(t *testing.T) {
	srv, shasum, downloads := serveAddonZip(t)
	for _, malformed := range []string{
		"sha256",
		"md5:" + strings.TrimPrefix(shasum, "sha256:"),
		shasum[:len(shasum)-1],
		shasum[:len(shasum)-1] + "z",
	} {
		pkg := lockfile.LockedPackage{Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Shasum: malformed}}
		target := filepath.Join(t.TempDir(), "addons")
		err := NewZipInstaller(store.New(t.TempDir())).Install(target, "acme/health-check", pkg)
		if err == nil || !strings.HasPrefix(err.Error(), "digest:") {
			t.Errorf("shasum %q: got error %v, want a digest error", malformed, err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("shasum %q: target was created: %v", malformed, err)
		}
	}
	if n := downloads.Load(); n != 0 {
		t.Fatalf("downloaded %d times, want 0", n)
	}
}

type zipEntry struct {
	name string
	mode fs.FileMode
	body string
}

func writeZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "addon.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUnzipRejectsUnsafeEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipEntry
		want    string
	}{
		{"symlink", []zipEntry{
			{name: "addon/__manifest__.py", body: "{}"},
			{name: "addon/static/x", mode: fs.ModeSymlink | 0777, body: "/etc/passwd"},
		}, "is a symlink"},
		{"parent component", []zipEntry{
			{name: "addon/../../evil.py", body: "x"},
		}, "contains a .. component"},
		{"absolute path", []zipEntry{
			{name: "/etc/addon/evil.py", body: "x"},
		}, "has an absolute path"},
		{"drive letter", []zipEntry{
			{name: "C:/addon/evil.py", body: "x"},
		}, "has an absolute path"},
		{"backslash", []zipEntry{
			{name: "addon\\..\\evil.py", body: "x"},
		}, "contains a backslash"},
		{"file at the top", []zipEntry{
			{name: "evil.py", body: "x"},
		}, "outside the root directory"},
		{"two root directories", []zipEntry{
			{name: "addon/__manifest__.py", body: "{}"},
			{name: "other/__manifest__.py", body: "{}"},
		}, "more than one root directory"},
		{"size bomb", []zipEntry{
			{name: "addon/a.bin", body: strings.Repeat("0", 600)},
			{name: "addon/b.bin", body: strings.Repeat("0", 600)},
		}, "more than the limit of 1000 bytes"},
		{"too many entries", []zipEntry{
			{name: "addon/a.py"}, {name: "addon/b.py"}, {name: "addon/c.py"}, {name: "addon/d.py"},
		}, "more than the limit of 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := t.TempDir()
			err := unzip(writeZip(t, tt.entries), dest, 1000, 3)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got error %v, want one containing %q", err, tt.want)
			}
			if _, err := os.Lstat(filepath.Join(dest, "addon", "static", "x")); err == nil {
				t.Fatal("symlink entry was extracted")
			}
		})
	}
}

func TestUnzipNestedAddon(t *testing.T) {
	path := writeZip(t, []zipEntry{
		{name: "addon/", mode: fs.ModeDir | 0700},
		{name: "addon/__manifest__.py", mode: 0600, body: "{}"},
		{name: "addon/models/partner.py", body: "x"},
		{name: "addon/static/src/js/deep/widget.js", body: "y"},
		{name: "addon/bin/run.sh", mode: 0750, body: "#!/bin/sh"},
	})
	dest := t.TempDir()
	if err := unzip(path, dest, maxZipBytes, maxZipEntries); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "addon", "static", "src", "js", "deep", "widget.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "y" {
		t.Fatalf("widget.js contains %q", got)
	}

	// Windows has no permission bits to check.
	if runtime.GOOS == "windows" {
		return
	}
	for rel, want := range map[string]fs.FileMode{
		"addon":                   0755,
		"addon/models":            0755,
		"addon/__manifest__.py":   0644,
		"addon/models/partner.py": 0644,
		"addon/bin/run.sh":        0755,
	} {
		info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s has mode %v, want %v", rel, info.Mode().Perm(), want)
		}
	}
}
