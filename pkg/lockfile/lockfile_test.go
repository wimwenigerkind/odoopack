package lockfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/json/jsontext"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

func TestLoadOrNewWithoutLockfile(t *testing.T) {
	t.Chdir(t.TempDir())
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
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, FileName)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(`{"packages":{"../evil":{"version":"19.0.1.0.0","module":".._evil","dist":{"type":"git","url":"https://example.com/x.git"}}}}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadOrNew()
	if err == nil || !strings.Contains(err.Error(), FileName) || !strings.Contains(err.Error(), "../evil") {
		t.Fatalf("got error %v, want one naming %s and the invalid name", err, path)
	}
}

// useLock points the lock setting at a lockfile in a temporary directory that
// holds content.
func useLock(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, FileName)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadWithoutFormatVersionIsOne(t *testing.T) {
	useLock(t, `{"content_hash":"sha256:00","packages":{}}`)

	lf, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if lf.FormatVersion != 1 {
		t.Fatalf("got format_version %d, want 1", lf.FormatVersion)
	}
}

func TestLoadRejectsNewerFormatVersion(t *testing.T) {
	path := useLock(t, fmt.Sprintf(`{"format_version":%d,"content_hash":"sha256:00","packages":{}}`, FormatVersion+1))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), FileName) || !strings.Contains(err.Error(), "upgrade odoopack") {
		t.Fatalf("got error %v, want one naming %s and asking to upgrade", err, path)
	}
}

func TestSaveWritesFormatVersionFirst(t *testing.T) {
	path := useLock(t, `{"content_hash":"sha256:00","packages":{}}`)

	lf, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(lf); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("{\n  \"format_version\": %d,\n", FormatVersion); !bytes.HasPrefix(data, []byte(want)) {
		t.Fatalf("lockfile does not start with %q:\n%s", want, data)
	}
}

func TestUnknownIndexKeysKeepContentHash(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	plain := manifest.Indexes{{Name: "company", Url: "https://registry.example.com", Type: "registry"}}
	extended := manifest.Indexes{{Name: "company", Url: "https://registry.example.com", Type: "registry", Unknown: map[string]jsontext.Value{"priority": jsontext.Value(`10`)}}}
	packages := map[string]LockedPackage{"acme/health-check": {Version: "19.0.1.0.0"}}

	hash, err := ComputeHash(require, plain, "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if stale, err := IsStale(require, extended, "19.0", packages, hash); err != nil || stale {
		t.Fatalf("lockfile is stale (%v) once an index gains an unknown key", err)
	}
}

func TestLockPackageRequiresIntegrity(t *testing.T) {
	const zipDigest = "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543"
	const commit = "6b6c390a387c81ee604b602e49ca6c35646cc619"
	tests := []struct {
		name    string
		lookup  index.AddonVersion
		wantErr string
	}{
		{"zip with digest", index.AddonVersion{Type: "zip", Digest: zipDigest}, ""},
		{"zip without digest", index.AddonVersion{Type: "zip"}, "zip has no digest"},
		{"zip with md5", index.AddonVersion{Type: "zip", Digest: "md5:00"}, "unsupported algorithm"},
		{"git with commit", index.AddonVersion{Type: "git", Reference: commit}, ""},
		{"git with sha256 commit", index.AddonVersion{Type: "git", Reference: strings.Repeat("a", 64)}, ""},
		{"git without reference", index.AddonVersion{Type: "git"}, "git has no reference"},
		{"git with tag", index.AddonVersion{Type: "git", Reference: "19.0.1.0.0"}, "must be a full commit id"},
		{"git with uppercase commit", index.AddonVersion{Type: "git", Reference: strings.ToUpper(commit)}, "must be a full commit id"},
		{"unknown type", index.AddonVersion{Type: "svn", Digest: zipDigest}, `type "svn"`},
		{"zip over http", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "http://registry.example.com/zipball/x"}, "must use https"},
		{"zip over http to localhost", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "http://localhost:8080/zipball/x"}, ""},
		{"zip over http to loopback", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "http://127.0.0.1:8080/zipball/x"}, ""},
		{"zip with relative url", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "/zipball/x"}, "not an absolute url"},
		{"zip with credentials over http", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "http://user:secret@registry.example.com/x"}, "must not contain credentials"},
		{"zip with credentials", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "https://user:secret@cdn.example.com/x.zip"}, "must not contain credentials"},
		{"zip with token as user", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "https://secret@cdn.example.com/x.zip"}, "must not contain credentials"},
		{"zip with signed query", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "https://cdn.example.com/x.zip?X-Amz-Signature=secret"}, "must not have a query"},
		{"zip with fragment", index.AddonVersion{Type: "zip", Digest: zipDigest, Repository: "https://cdn.example.com/x.zip#secret"}, "must not have a fragment"},
		{"git over ssh with user", index.AddonVersion{Type: "git", Reference: commit, Repository: "ssh://git@github.com/acme/x.git"}, ""},
		{"git scp style", index.AddonVersion{Type: "git", Reference: commit, Repository: "git@github.com:acme/x.git"}, ""},
		{"git over https with token", index.AddonVersion{Type: "git", Reference: commit, Repository: "https://secret@github.com/acme/x.git"}, "must not contain credentials"},
		{"git over ssh with password", index.AddonVersion{Type: "git", Reference: commit, Repository: "ssh://git:secret@github.com/acme/x.git"}, "must not contain credentials"},
		{"git with query", index.AddonVersion{Type: "git", Reference: commit, Repository: "https://github.com/acme/x.git?token=secret"}, "must not have a query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.lookup.Name = "acme/health-check"
			tt.lookup.Version = "19.0.1.0.0"
			if tt.lookup.Repository == "" {
				tt.lookup.Repository = "https://registry.example.com/zipball/x"
			}
			pkg, err := lockPackage(tt.lookup, true, nil, nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if pkg.Module != "acme_health_check" || pkg.Dist.Type != tt.lookup.Type || pkg.Dist.Digest != tt.lookup.Digest || pkg.Dist.Reference != tt.lookup.Reference {
					t.Fatalf("locked %+v from %+v", pkg, tt.lookup)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "acme/health-check 19.0.1.0.0") {
				t.Fatalf("got error %v, want one naming the package and containing %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error %v leaks the credentials", err)
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
		hash, err := ComputeHash(require, nil, from)
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

func TestCheckModules(t *testing.T) {
	tests := []struct {
		name     string
		packages []string
		wantErr  string
	}{
		{"none", []string{"acme/a-b", "acme/a-c", "acme/b"}, ""},
		{"empty", nil, ""},
		{"dash and underscore", []string{"acme/a-b", "acme/a_b"}, "packages acme/a-b and acme/a_b both install to the addon directory acme_a_b"},
		{"slash and underscore", []string{"acme-a/b", "acme_a/b"}, "packages acme-a/b and acme_a/b both install to the addon directory acme_a_b"},
		{"three ways", []string{"acme/a_b", "acme-a/b", "acme/a-b", "acme/c"}, "both install to the addon directory acme_a_b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packages := make(map[string]LockedPackage)
			for _, pkg := range tt.packages {
				packages[pkg] = LockedPackage{Module: name.ModuleName(pkg)}
			}
			err := CheckModules(packages)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got error %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadRejectsModuleCollision(t *testing.T) {
	path := useLock(t, `{"content_hash":"sha256:00","packages":{"acme/a-b":{"version":"19.0.1.0.0","module":"acme_a_b","dist":{"type":"git","url":"https://example.com/x.git"}},"acme/a_b":{"version":"19.0.1.0.0","module":"acme_a_b","dist":{"type":"git","url":"https://example.com/y.git"}}}}`)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), FileName) || !strings.Contains(err.Error(), "acme/a-b and acme/a_b") {
		t.Fatalf("got error %v, want one naming %s and both packages", err, path)
	}
}

func TestLoadRejectsDuplicateKeys(t *testing.T) {
	useLock(t, `{"format_version":1,"content_hash":"sha256:00","packages":{"acme/x":{"version":"19.0.1.0.0","module":"acme_x","version":"19.0.2.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`)

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("got error %v, want one about the duplicate key", err)
	}
}

// TestComputeHashIsCanonical pins content_hash to its definition: the
// RFC 8785 form of odoo_series, require and the indexes by URL, which a tool
// other than odoopack computes the same way.
func TestComputeHashIsCanonical(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0", "acme/base-tools": "19.0.1.0.0"}
	indexes := manifest.Indexes{
		{Name: "public", Url: "https://odoopack.example.com", Type: "registry"},
		{Name: "company", Url: "https://registry.example.com", Type: "registry"},
	}
	canonical := `{"indexes":[{"type":"registry","url":"https://odoopack.example.com"},{"type":"registry","url":"https://registry.example.com"}],"odoo_series":"19.0","require":{"acme/base-tools":"19.0.1.0.0","acme/health-check":"19.0"}}`

	got, err := ComputeHash(require, indexes, "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(canonical))); got != want {
		t.Fatalf("got %s, want %s, the sha256 of %s", got, want, canonical)
	}

	renamed := manifest.Indexes{
		{Name: "company-2", Url: "https://registry.example.com", Type: "registry"},
		{Name: "public", Url: "https://odoopack.example.com", Type: "registry"},
	}
	if again, err := ComputeHash(require, renamed, "19.0"); err != nil || again != got {
		t.Fatalf("renaming and reordering the indexes changed the hash to %s (%v)", again, err)
	}
}

func TestMissingRequiredPackageIsStale(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0", "acme/base-tools": "19.0"}
	hash, err := ComputeHash(require, nil, "19.0")
	if err != nil {
		t.Fatal(err)
	}
	packages := map[string]LockedPackage{"acme/health-check": {Version: "19.0.1.0.0"}}
	if stale, err := IsStale(require, nil, "19.0", packages, hash); err != nil || !stale {
		t.Fatalf("lockfile without a required package is not stale (%v)", err)
	}
}

func TestLoadRejectsInvalidModule(t *testing.T) {
	for _, module := range []string{"", "../evil", "acme/x", "Acme_X"} {
		useLock(t, `{"content_hash":"sha256:00","packages":{"acme/x":{"version":"19.0.1.0.0","module":"`+module+`","dist":{"type":"git","url":"https://example.com/x.git"}}}}`)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "acme/x") {
			t.Errorf("module %q: got error %v, want one naming the package", module, err)
		}
	}
}

func TestComputeHashOfEmptyManifest(t *testing.T) {
	canonical := `{"indexes":[],"odoo_series":"19.0","require":{}}`
	want := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(canonical)))

	for _, require := range []map[string]string{nil, {}} {
		for _, indexes := range []manifest.Indexes{nil, {}} {
			got, err := ComputeHash(require, indexes, "19.0")
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("require %#v and indexes %#v hash to %s, want %s, the sha256 of %s", require, indexes, got, want, canonical)
			}
		}
	}
}

func TestSaveKeepsUnknownPackageKeys(t *testing.T) {
	path := useLock(t, `{
  "format_version": 1,
  "content_hash": "sha256:00",
  "signature": "whole-file",
  "packages": {
    "acme/x": {
      "version": "19.0.1.0.0",
      "module": "acme_x",
      "dist": {
        "type": "git",
        "url": "https://example.com/x.git",
        "reference": "6b6c390a387c81ee604b602e49ca6c35646cc619",
        "size": 1234
      },
      "direct": true,
      "provenance": {"builder": "ci"}
    }
  }
}`)

	lf, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(lf); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{`"size": 1234`, `"provenance": {`, `"builder": "ci"`} {
		if !bytes.Contains(data, []byte(kept)) {
			t.Errorf("lockfile lost %s:\n%s", kept, data)
		}
	}
	if bytes.Contains(data, []byte(`"signature"`)) {
		t.Errorf("lockfile kept a top-level key it does not know:\n%s", data)
	}
}
