package lockfile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

func TestLoadOrNewWithoutLockfile(t *testing.T) {
	viper.Set("lock", filepath.Join(t.TempDir(), "odoopack.lock"))
	t.Cleanup(viper.Reset)

	lf, err := LoadOrNew()
	if err != nil {
		t.Fatal(err)
	}
	if lf.Packages == nil || len(lf.Packages) != 0 {
		t.Fatalf("got packages %v, want an empty map", lf.Packages)
	}
}

func TestLoadOrNewRejectsInvalidName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odoopack.lock")
	viper.Set("lock", path)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(`{"packages":{"../evil":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadOrNew()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "../evil") {
		t.Fatalf("got error %v, want one naming %s and the invalid name", err, path)
	}
}

// useLock points the lock setting at a lockfile in a temporary directory that
// holds content.
func useLock(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "odoopack.lock")
	viper.Set("lock", path)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadWithoutFormatIsFormatOne(t *testing.T) {
	useLock(t, `{"content_hash":"sha256:00","packages":{}}`)

	lf, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if lf.Format != 1 {
		t.Fatalf("got format %d, want 1", lf.Format)
	}
}

func TestLoadRejectsNewerFormat(t *testing.T) {
	path := useLock(t, fmt.Sprintf(`{"format":%d,"content_hash":"sha256:00","packages":{}}`, Format+1))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "upgrade odoopack") {
		t.Fatalf("got error %v, want one naming %s and asking to upgrade", err, path)
	}
}

func TestSaveWritesFormatFirst(t *testing.T) {
	path := useLock(t, "")

	if err := Save(LockFile{Packages: map[string]LockedPackage{}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("{\n  \"format\": %d,\n", Format); !bytes.HasPrefix(data, []byte(want)) {
		t.Fatalf("lockfile does not start with %q:\n%s", want, data)
	}
}

func TestFormatKeepsContentHash(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	indexes := manifest.Indexes{"company": {Url: "https://registry.example.com", Type: "odoopack"}}
	packages := map[string]LockedPackage{
		"acme/health-check": {Version: "19.0.1.0.0", Dist: Dist{Type: "git", URL: "https://example.com/x.git"}, Direct: true},
	}
	hash, err := ComputeHash(require, indexes, packages)
	if err != nil {
		t.Fatal(err)
	}
	useLock(t, `{"content_hash":"`+hash+`","packages":{"acme/health-check":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"},"direct":true}}}`)

	old, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(old); err != nil {
		t.Fatal(err)
	}
	saved, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	got, err := ComputeHash(require, indexes, saved.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if got != hash || saved.ContentHash != hash {
		t.Fatalf("hash after gaining the format is %s (stored %s), want %s", got, saved.ContentHash, hash)
	}
}
