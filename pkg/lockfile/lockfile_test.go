package lockfile

import (
	"bytes"
	"crypto/sha256"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
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

func TestLoadReadsLegacyFormat(t *testing.T) {
	path := useLock(t, fmt.Sprintf(`{"format":%d,"content_hash":"sha256:00","packages":{}}`, FormatVersion+1))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "upgrade odoopack") {
		t.Fatalf("got error %v, want the legacy format key to be checked like format_version", err)
	}
}

func TestLoadRejectsNewerFormatVersion(t *testing.T) {
	path := useLock(t, fmt.Sprintf(`{"format_version":%d,"content_hash":"sha256:00","packages":{}}`, FormatVersion+1))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "upgrade odoopack") {
		t.Fatalf("got error %v, want one naming %s and asking to upgrade", err, path)
	}
}

func TestSaveWritesFormatVersionFirst(t *testing.T) {
	path := useLock(t, `{"format":1,"content_hash":"sha256:00","packages":{}}`)

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
	if bytes.Contains(data, []byte(`"format":`)) {
		t.Fatalf("lockfile still has the legacy format key:\n%s", data)
	}
}

func TestFormatKeepsContentHash(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	indexes := manifest.Indexes{{Name: "company", Url: "https://registry.example.com", Type: "odoopack"}}
	packages := map[string]LockedPackage{
		"acme/health-check": {Version: "19.0.1.0.0", Dist: Dist{Type: "git", URL: "https://example.com/x.git"}, Direct: true},
	}
	hash, err := packagesHash(require, indexes, "19.0", packages)
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

	got, err := packagesHash(require, indexes, "19.0", saved.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if got != hash || saved.ContentHash != hash {
		t.Fatalf("hash after gaining the format is %s (stored %s), want %s", got, saved.ContentHash, hash)
	}
}

// legacyLock is a lockfile with the keys releases before the renaming wrote,
// and legacyHash its content_hash as those releases computed it.
const (
	legacyLock        = `{"format":1,"content_hash":"sha256:522cfae8eb38a569f835d291f2e53ae801859e481a6b87f5f4c809d2124043bf","packages":{"acme/health-check":{"version":"19.0.1.0.0","dist":{"type":"zip","url":"https://registry.example.com/z","reference":"6b6c390a387c81ee604b602e49ca6c35646cc619","shasum":"sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543"},"direct":true,"depends":["acme/base-tools"],"external":["base"],"registry":"https://registry.example.com"}}}`
	legacyContentHash = "sha256:522cfae8eb38a569f835d291f2e53ae801859e481a6b87f5f4c809d2124043bf"
)

func TestLoadReadsLegacyKeysAndKeepsContentHash(t *testing.T) {
	path := useLock(t, legacyLock)

	lf, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	pkg := lf.Packages["acme/health-check"]
	if pkg.Dist.Digest == "" || pkg.IndexURL != "https://registry.example.com" || strings.Join(pkg.ExternalModules, ",") != "base" {
		t.Fatalf("legacy keys not read: %+v", pkg)
	}

	require := map[string]string{"acme/health-check": "19.0"}
	indexes := manifest.Indexes{{Name: "company", Url: "https://registry.example.com", Type: "registry"}}
	stale, err := IsStale(require, indexes, "19.0", lf.Packages, lf.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("a lockfile with the legacy keys is stale")
	}
	if got, err := packagesHash(require, indexes, "19.0", lf.Packages); err != nil || got != legacyContentHash {
		t.Fatalf("got hash %s (%v), want %s", got, err, legacyContentHash)
	}

	if err := Save(lf); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"format":`, `"shasum":`, `"external":`, `"registry":`} {
		if bytes.Contains(data, []byte(key)) {
			t.Errorf("lockfile still has the legacy key %s:\n%s", key, data)
		}
	}
	for _, key := range []string{`"format_version":`, `"digest":`, `"external_modules":`, `"index_url":`} {
		if !bytes.Contains(data, []byte(key)) {
			t.Errorf("lockfile lacks %s:\n%s", key, data)
		}
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
	named, err := namedHash(require, plain, "19.0", packages)
	if err != nil {
		t.Fatal(err)
	}
	if stale, err := IsStale(require, extended, "19.0", packages, named); err != nil || stale {
		t.Fatalf("lockfile hashed by index name is stale (%v) once an index gains an unknown key", err)
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
		{"zip without digest", index.AddonVersion{Type: "zip"}, "zip has no digest"},
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
				if pkg.Dist.Type != tt.lookup.Type || pkg.Dist.Digest != tt.lookup.Shasum || pkg.Dist.Reference != tt.lookup.Reference {
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

func previousHash(t *testing.T, require map[string]string, indexes manifest.Indexes, packages map[string]LockedPackage) string {
	t.Helper()
	// The previous odoopack kept indexes as an object by name.
	type index struct {
		Url  string `json:"url"`
		Type string `json:"type"`
	}
	byName := make(map[string]index, len(indexes))
	for _, idx := range indexes {
		byName[idx.Name] = index{Url: idx.Url, Type: idx.Type}
	}
	payload := struct {
		Require  map[string]string        `json:"require"`
		Indexes  map[string]index         `json:"indexes"`
		Packages map[string]LockedPackage `json:"packages"`
	}{Require: require, Indexes: byName, Packages: packages}

	data, err := jsonv1.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data))
}

func TestPreviousHashIsNotStale(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	indexes := manifest.Indexes{{Name: "company", Url: "https://registry.example.com", Type: "registry"}}
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

func TestCheckModuleCollisions(t *testing.T) {
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
				packages[pkg] = LockedPackage{}
			}
			err := CheckModuleCollisions(packages)
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
	path := useLock(t, `{"content_hash":"sha256:00","packages":{"acme/a-b":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"}},"acme/a_b":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/y.git"}}}}`)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "acme/a-b and acme/a_b") {
		t.Fatalf("got error %v, want one naming %s and both packages", err, path)
	}
}

func TestSaveWritesWhatV1Wrote(t *testing.T) {
	path := useLock(t, "")
	lf := LockFile{
		ContentHash: "sha256:00",
		Packages: map[string]LockedPackage{
			"acme/health-check": {
				Version:         "19.0.1.0.0",
				Dist:            Dist{Type: "zip", URL: "https://registry.example.com/z", Reference: "6b6c390a387c81ee604b602e49ca6c35646cc619", Digest: "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543"},
				Direct:          true,
				Depends:         []string{"acme/base-tools"},
				ExternalModules: []string{"base"},
				IndexURL:        "https://registry.example.com",
			},
			"acme/base-tools": {Version: "dev-19.0", Dist: Dist{Type: "git", URL: "https://example.com/x.git", Reference: "6b6c390a387c81ee604b602e49ca6c35646cc619"}},
		},
	}
	if err := Save(lf); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lf.FormatVersion = FormatVersion
	want, err := jsonv1.MarshalIndent(lf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got\n%s\nwant what encoding/json v1 wrote\n%s", got, want)
	}
}

func TestLoadRejectsDuplicateKeys(t *testing.T) {
	useLock(t, `{"format_version":1,"content_hash":"sha256:00","packages":{"acme/x":{"version":"19.0.1.0.0","version":"19.0.2.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`)

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
		{Name: "public", Url: "https://ODOOPACK.example.com/", Type: "registry"},
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
