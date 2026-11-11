package installer

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
