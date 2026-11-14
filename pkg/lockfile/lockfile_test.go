package lockfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/index"
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
	hash, err := ComputeHash(require, indexes, "19.0", packages)
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

	got, err := ComputeHash(require, indexes, "19.0", saved.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if got != hash || saved.ContentHash != hash {
		t.Fatalf("hash after gaining the format is %s (stored %s), want %s", got, saved.ContentHash, hash)
	}
}

func TestLockPackageRequiresIntegrity(t *testing.T) {
	const shasum = "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543"
	const commit = "6b6c390a387c81ee604b602e49ca6c35646cc619"
	tests := []struct {
		name    string
		lookup  index.AddonVersion
		wantErr string
	}{
		{"zip with shasum", index.AddonVersion{Type: "zip", Shasum: shasum}, ""},
		{"zip without shasum", index.AddonVersion{Type: "zip"}, "zip has no shasum"},
		{"zip with md5", index.AddonVersion{Type: "zip", Shasum: "md5:00"}, "unsupported algorithm"},
		{"git with commit", index.AddonVersion{Type: "git", Reference: commit}, ""},
		{"git with sha256 commit", index.AddonVersion{Type: "git", Reference: strings.Repeat("a", 64)}, ""},
		{"git without reference", index.AddonVersion{Type: "git"}, "git has no reference"},
		{"git with tag", index.AddonVersion{Type: "git", Reference: "19.0.1.0.0"}, "must be a full commit id"},
		{"git with uppercase commit", index.AddonVersion{Type: "git", Reference: strings.ToUpper(commit)}, "must be a full commit id"},
		{"unknown type", index.AddonVersion{Type: "svn", Shasum: shasum}, `type "svn"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.lookup.Name = "acme/health-check"
			tt.lookup.Version = "19.0.1.0.0"
			tt.lookup.Repository = "https://registry.example.com/zipball/x"
			pkg, err := lockPackage(tt.lookup, true, nil, nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if pkg.Dist.Type != tt.lookup.Type || pkg.Dist.Shasum != tt.lookup.Shasum || pkg.Dist.Reference != tt.lookup.Reference {
					t.Fatalf("locked %+v from %+v", pkg.Dist, tt.lookup)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "acme/health-check 19.0.1.0.0") {
				t.Fatalf("got error %v, want one naming the package and containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestChangedSeriesMakesLockfileStale(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	packages := map[string]LockedPackage{
		"acme/health-check": {Version: "19.0.1.0.0", Dist: Dist{Type: "git", URL: "https://example.com/x.git"}, Direct: true},
	}

	for _, from := range []string{"19.0", ""} {
		hash, err := ComputeHash(require, nil, from, packages)
		if err != nil {
			t.Fatal(err)
		}
		if stale, err := IsStale(require, nil, from, packages, hash); err != nil || stale {
			t.Fatalf("series %q: lockfile is stale (%v) without a change", from, err)
		}
		if stale, err := IsStale(require, nil, "18.0", packages, hash); err != nil || !stale {
			t.Fatalf("series %q: lockfile is not stale (%v) after changing the series to 18.0", from, err)
		}
	}
}

func previousHash(t *testing.T, require map[string]string, indexes manifest.Indexes, packages map[string]LockedPackage) string {
	t.Helper()
	payload := struct {
		Require  map[string]string        `json:"require"`
		Indexes  manifest.Indexes         `json:"indexes"`
		Packages map[string]LockedPackage `json:"packages"`
	}{Require: require, Indexes: indexes, Packages: packages}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data))
}

func TestPreviousHashIsNotStale(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	indexes := manifest.Indexes{"company": {Url: "https://registry.example.com", Type: "registry"}}
	packages := map[string]LockedPackage{
		"acme/health-check": {Version: "19.0.1.0.0", Dist: Dist{Type: "git", URL: "https://example.com/x.git"}, Direct: true},
	}
	hash := previousHash(t, require, indexes, packages)

	if stale, err := IsStale(require, indexes, "19.0", packages, hash); err != nil || stale {
		t.Fatalf("lockfile from the previous odoopack is stale (%v)", err)
	}
	changed := map[string]string{"acme/health-check": "19.0.2"}
	if stale, err := IsStale(changed, indexes, "19.0", packages, hash); err != nil || !stale {
		t.Fatalf("lockfile from the previous odoopack is not stale (%v) after changing require", err)
	}
}
