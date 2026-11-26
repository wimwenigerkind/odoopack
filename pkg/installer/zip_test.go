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

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/buildinfo"
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
	srv, zipDigest, downloads := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Module: "acme_health_check", Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Digest: zipDigest}}
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
	srv, zipDigest, downloads := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Module: "acme_health_check", Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Digest: zipDigest}}
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

func TestZipInstallRejectsDigestMismatch(t *testing.T) {
	srv, _, _ := serveAddonZip(t)
	wrong := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("other")))
	pkg := lockfile.LockedPackage{Module: "acme_health_check", Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Digest: wrong}}
	storeDir := t.TempDir()
	inst := NewZipInstaller(store.New(storeDir))

	if err := inst.Install(t.TempDir(), "acme/health-check", pkg); err == nil {
		t.Fatal("expected a digest mismatch")
	}
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	// The store's lock file stays; nothing else may.
	if len(entries) != 1 || entries[0].Name() != ".lock" {
		t.Fatalf("store keeps %v after a mismatch, want only the lock file", entries)
	}
}

func TestZipInstallRejectsMalformedDigest(t *testing.T) {
	srv, zipDigest, downloads := serveAddonZip(t)
	for _, malformed := range []string{
		"sha256",
		"md5:" + strings.TrimPrefix(zipDigest, "sha256:"),
		zipDigest[:len(zipDigest)-1],
		zipDigest[:len(zipDigest)-1] + "z",
	} {
		pkg := lockfile.LockedPackage{Module: "acme_health_check", Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Digest: malformed}}
		target := filepath.Join(t.TempDir(), "addons")
		err := NewZipInstaller(store.New(t.TempDir())).Install(target, "acme/health-check", pkg)
		if err == nil || !strings.Contains(err.Error(), "digest:") {
			t.Errorf("zipDigest %q: got error %v, want a digest error", malformed, err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("zipDigest %q: target was created: %v", malformed, err)
		}
	}
	if n := downloads.Load(); n != 0 {
		t.Fatalf("downloaded %d times, want 0", n)
	}
}

// A hand-edited lockfile may lack the digest.
func TestZipInstallRequiresDigest(t *testing.T) {
	srv, _, downloads := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Module: "acme_health_check", Dist: lockfile.Dist{Type: "zip", URL: srv.URL}}
	for _, st := range []*store.Store{store.New(t.TempDir()), nil} {
		target := filepath.Join(t.TempDir(), "addons")
		err := NewZipInstaller(st).Install(target, "acme/health-check", pkg)
		if err == nil || !strings.Contains(err.Error(), "has no digest") || !strings.Contains(err.Error(), "odoopack update acme/health-check") {
			t.Errorf("store %v: got error %v, want one about the missing digest", st != nil, err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("store %v: target was created: %v", st != nil, err)
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

func TestDownloadSendsUserAgent(t *testing.T) {
	version := buildinfo.Version
	buildinfo.Version = "1.2.3"
	t.Cleanup(func() { buildinfo.Version = version })

	var userAgent atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent.Store(r.UserAgent())
	}))
	t.Cleanup(srv.Close)

	tmp, err := downloadToTmp(srv.URL, maxZipballBytes)
	if err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()
	_ = os.Remove(tmp.Name())
	if got := userAgent.Load(); got != "odoopack/1.2.3" {
		t.Fatalf("got User-Agent %v, want odoopack/1.2.3", got)
	}
}

func TestDownloadLeavesNoTempFileOnError(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		url     string
		auth    string
		wantErr string
	}{
		{name: "network", url: closed.URL, wantErr: "connect"},
		{name: "status", handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }, wantErr: "bad status 404"},
		{name: "truncated", handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("short"))
		}, wantErr: "unexpected EOF"},
		{name: "limit", handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(make([]byte, 11)) }, wantErr: "size limit"},
		{name: "token over http", url: "http://registry.example.com/zipball/1", auth: `{"bearer":{"registry.example.com":"secret"}}`, wantErr: "plain http"},
		{name: "malformed auth", url: "https://registry.example.com/zipball/1", auth: `{"bearer":`, wantErr: "ODOOPACK_AUTH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("TMPDIR", tmpDir)
			viper.Set("auth", tc.auth)
			t.Cleanup(viper.Reset)
			url := tc.url
			if tc.handler != nil {
				srv := httptest.NewServer(tc.handler)
				t.Cleanup(srv.Close)
				url = srv.URL
			}

			tmp, err := downloadToTmp(url, 10)
			if err == nil {
				_ = tmp.Close()
				t.Fatal("download succeeded")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got error %v, want one containing %q", err, tc.wantErr)
			}
			left, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(left) != 0 {
				t.Fatalf("left %v in the temp directory", left)
			}
		})
	}
}

func TestZipInstallUsesLockedModule(t *testing.T) {
	srv, zipDigest, _ := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Module: "health_check", Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Digest: zipDigest}}
	target := t.TempDir()

	if err := NewZipInstaller(nil).Install(target, "acme/health-check", pkg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "health_check", "__manifest__.py")); err != nil {
		t.Fatalf("not installed into the locked module: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "acme_health_check")); !os.IsNotExist(err) {
		t.Fatalf("installed into the directory derived from the package name: %v", err)
	}
}

func TestZipInstallRejectsInvalidModule(t *testing.T) {
	srv, zipDigest, downloads := serveAddonZip(t)
	pkg := lockfile.LockedPackage{Module: "../evil", Dist: lockfile.Dist{Type: "zip", URL: srv.URL, Digest: zipDigest}}

	if err := NewZipInstaller(nil).Install(t.TempDir(), "acme/health-check", pkg); err == nil {
		t.Fatal("installed a package with an invalid module")
	}
	if n := downloads.Load(); n != 0 {
		t.Fatalf("downloaded %d times before refusing the module", n)
	}
}
