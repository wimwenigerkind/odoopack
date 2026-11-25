package lockfile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

// registry serves addons by name, answers 404 for any other, and counts the
// requests it gets.
func registry(t *testing.T, addons ...registryapi.Addon) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		for _, addon := range addons {
			if r.URL.Path == "/registry/v1/addons/"+addon.Name {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(addon)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

func addon(name string, depends ...string) registryapi.Addon {
	v := registryapi.Version{
		Version: "19.0.1.0.0",
		Type:    registryapi.TypeZip,
		URL:     "https://registry.example.com/registry/v1/zipball/f1740dd9-ca0e-42fd-a63b-b553d3fd3882/ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543",
		Shasum:  "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543",
	}
	for _, d := range depends {
		v.Depends = append(v.Depends, registryapi.Dep{Module: strings.ReplaceAll(strings.Split(d, "/")[1], "-", "_"), Package: d, Access: registryapi.AccessOK})
	}
	return registryapi.Addon{Name: name, Versions: []registryapi.Version{v}}
}

func TestRecomputeHashRecordsIndex(t *testing.T) {
	first, _ := registry(t, addon("acme/health-check", "acme/base-tools"))
	second, _ := registry(t, addon("acme/base-tools"))
	viper.Set("default_index_url", "")
	t.Cleanup(viper.Reset)
	indexes := manifest.Indexes{
		"a": {Url: first, Type: "registry"},
		"b": {Url: second, Type: "registry"},
	}

	lf, err := RecomputeHash(map[string]string{"acme/health-check": "19.0"}, indexes, "19.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := lf.Packages["acme/health-check"].Registry; got != first {
		t.Errorf("acme/health-check locked to %q, want index a (%s)", got, first)
	}
	if got := lf.Packages["acme/base-tools"].Registry; got != second {
		t.Errorf("acme/base-tools locked to %q, want index b (%s)", got, second)
	}
}

func TestRecomputeHashRecordsImplicitDefault(t *testing.T) {
	public, _ := registry(t, addon("acme/health-check"))
	viper.Set("default_index_url", public)
	t.Cleanup(viper.Reset)

	lf, err := RecomputeHash(map[string]string{"acme/health-check": "19.0"}, nil, "19.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := lf.Packages["acme/health-check"].Registry; got != public {
		t.Fatalf("locked to %q, want the default index %s", got, public)
	}
}

func TestRecomputeHashResolvesLockedFromItsIndex(t *testing.T) {
	first, firstHits := registry(t, addon("acme/health-check", "acme/base-tools"), addon("acme/base-tools"))
	second, _ := registry(t, addon("acme/health-check", "acme/base-tools"), addon("acme/base-tools"))
	viper.Set("default_index_url", "")
	t.Cleanup(viper.Reset)
	indexes := manifest.Indexes{
		"a": {Url: first, Type: "registry"},
		"b": {Url: second, Type: "registry"},
	}
	locked := map[string]LockedPackage{
		"acme/health-check": {Index: "b"},
		"acme/base-tools":   {Index: "b"},
	}

	lf, err := RecomputeHash(map[string]string{"acme/health-check": "19.0"}, indexes, "19.0", locked)
	if err != nil {
		t.Fatal(err)
	}
	for name, pkg := range lf.Packages {
		if pkg.Registry != second || pkg.Index != "" {
			t.Errorf("%s locked to %q (index %q), want only the URL of b (%s)", name, pkg.Registry, pkg.Index, second)
		}
	}
	if n := firstHits.Load(); n != 0 {
		t.Fatalf("index a got %d request(s) for packages locked to b", n)
	}
}

func TestRecomputeHashWithLockfileWithoutIndex(t *testing.T) {
	first, _ := registry(t)
	second, _ := registry(t, addon("acme/health-check"))
	viper.Set("default_index_url", "")
	indexes := manifest.Indexes{
		"a": {Url: first, Type: "registry"},
		"b": {Url: second, Type: "registry"},
	}
	useLock(t, `{"content_hash":"sha256:00","packages":{"acme/health-check":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"},"direct":true}}}`)
	old, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	lf, err := RecomputeHash(map[string]string{"acme/health-check": "19.0"}, indexes, "19.0", old.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if got := lf.Packages["acme/health-check"].Registry; got != second {
		t.Fatalf("locked to %q, want index b (%s)", got, second)
	}
}

func TestLockfileWithoutIndexKeepsContentHash(t *testing.T) {
	require := map[string]string{"acme/health-check": "19.0"}
	indexes := manifest.Indexes{"company": {Url: "https://registry.example.com", Type: "registry"}}
	packages := map[string]LockedPackage{
		"acme/health-check": {Version: "19.0.1.0.0", Dist: Dist{Type: "git", URL: "https://example.com/x.git"}, Direct: true},
	}
	hash, err := ComputeHash(require, indexes, "19.0", packages)
	if err != nil {
		t.Fatal(err)
	}
	path := useLock(t, `{"format_version":1,"content_hash":"`+hash+`","packages":{"acme/health-check":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"},"direct":true}}}`)

	old, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if stale, err := IsStale(require, indexes, "19.0", old.Packages, old.ContentHash); err != nil || stale {
		t.Fatalf("lockfile without index is stale (%v, %v)", stale, err)
	}
	if err := Save(old); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"index"`) {
		t.Fatalf("saving a loaded lockfile added the index field:\n%s", data)
	}
}

func release(version string, depends ...string) registryapi.Version {
	v := addon("acme/any", depends...).Versions[0]
	v.Version = version
	return v
}

// locked is a package locked to index a with the dist release serves.
func locked(version string, direct bool, depends ...string) LockedPackage {
	v := release(version)
	return LockedPackage{
		Version: version,
		Dist:    Dist{Type: v.Type, URL: v.URL, Shasum: v.Shasum},
		Direct:  direct,
		Depends: depends,
		Index:   "a",
	}
}

func singleIndex(t *testing.T, addons ...registryapi.Addon) (manifest.Indexes, *atomic.Int32) {
	t.Helper()
	url, hits := registry(t, addons...)
	viper.Set("default_index_url", "")
	t.Cleanup(viper.Reset)
	return manifest.Indexes{"a": {Url: url, Type: "registry"}}, hits
}

func TestUpdateResolvesTreeAndKeepsOthers(t *testing.T) {
	indexes, _ := singleIndex(t,
		registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{
			release("19.0.1.0.0", "acme/old-dep"),
			release("19.0.2.0.0", "acme/base-tools", "acme/shared"),
		}},
		registryapi.Addon{Name: "acme/base-tools", Versions: []registryapi.Version{release("19.0.1.0.0")}},
		registryapi.Addon{Name: "acme/old-dep", Versions: []registryapi.Version{release("19.0.1.0.0")}},
		registryapi.Addon{Name: "acme/shared", Versions: []registryapi.Version{release("19.0.1.0.0"), release("19.0.2.0.0")}},
		registryapi.Addon{Name: "acme/other", Versions: []registryapi.Version{release("19.0.1.0.0", "acme/shared"), release("19.0.2.0.0", "acme/shared")}},
	)
	require := map[string]string{"acme/health-check": "19.0", "acme/other": "19.0"}
	old := map[string]LockedPackage{
		"acme/health-check": locked("19.0.1.0.0", true, "acme/old-dep"),
		"acme/old-dep":      locked("19.0.1.0.0", false),
		"acme/other":        locked("19.0.1.0.0", true, "acme/shared"),
		"acme/shared":       locked("19.0.1.0.0", false),
	}

	lf, err := Update(require, indexes, "19.0", old, "acme/health-check")
	if err != nil {
		t.Fatal(err)
	}
	hc := lf.Packages["acme/health-check"]
	if hc.Version != "19.0.2.0.0" || !hc.Direct || hc.Registry != indexes["a"].Url || strings.Join(hc.Depends, ",") != "acme/base-tools,acme/shared" {
		t.Errorf("acme/health-check locked as %+v", hc)
	}
	if bt, ok := lf.Packages["acme/base-tools"]; !ok || bt.Direct {
		t.Errorf("new dependency acme/base-tools locked as %+v, %v", bt, ok)
	}
	if _, ok := lf.Packages["acme/old-dep"]; ok {
		t.Error("acme/old-dep is still locked, but nothing needs it")
	}
	// acme/shared is in the tree of the updated addon, acme/other is not.
	if got := lf.Packages["acme/shared"].Version; got != "19.0.2.0.0" {
		t.Errorf("acme/shared locked at %s, want 19.0.2.0.0", got)
	}
	if got := lf.Packages["acme/other"]; got.Version != "19.0.1.0.0" || !got.Direct {
		t.Errorf("acme/other locked as %+v, want it kept at 19.0.1.0.0", got)
	}
	if want, _ := ComputeHash(require, indexes, "19.0", lf.Packages); lf.ContentHash != want {
		t.Error("content hash does not match the packages")
	}
}

func TestRelockKeepsLockedAndDropsOrphans(t *testing.T) {
	indexes, hits := singleIndex(t)
	old := map[string]LockedPackage{
		"acme/health-check": locked("19.0.1.0.0", true, "acme/base-tools", "acme/shared"),
		"acme/base-tools":   locked("19.0.1.0.0", false),
		"acme/other":        locked("19.0.1.0.0", true, "acme/shared"),
		"acme/shared":       locked("19.0.1.0.0", false),
	}

	lf, err := Relock(map[string]string{"acme/other": "19.0"}, indexes, "19.0", old)
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Packages) != 2 || lf.Packages["acme/other"].Version == "" || lf.Packages["acme/shared"].Version == "" {
		t.Errorf("locked %v, want acme/other and acme/shared", lf.Packages)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("relocking locked packages sent %d request(s)", n)
	}
}

func TestRelockResolvesEntryWithoutDirect(t *testing.T) {
	indexes, _ := singleIndex(t,
		registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{release("19.0.1.0.0", "acme/base-tools")}},
		registryapi.Addon{Name: "acme/base-tools", Versions: []registryapi.Version{release("19.0.1.0.0")}},
	)
	// An update before direct and depends were kept wrote entries like this.
	old := map[string]LockedPackage{"acme/health-check": locked("19.0.1.0.0", false)}

	lf, err := Relock(map[string]string{"acme/health-check": "19.0"}, indexes, "19.0", old)
	if err != nil {
		t.Fatal(err)
	}
	if hc := lf.Packages["acme/health-check"]; !hc.Direct || len(hc.Depends) != 1 {
		t.Errorf("acme/health-check locked as %+v", hc)
	}
	if _, ok := lf.Packages["acme/base-tools"]; !ok {
		t.Error("acme/base-tools is not locked")
	}
}

func TestDirectVersionWinsOverDependency(t *testing.T) {
	indexes, _ := singleIndex(t,
		registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{release("19.0.1.0.0", "acme/base-tools")}},
		registryapi.Addon{Name: "acme/base-tools", Versions: []registryapi.Version{release("19.0.1.0.0"), release("19.0.2.0.0")}},
	)
	require := map[string]string{"acme/health-check": "19.0", "acme/base-tools": "19.0.1.0.0"}

	full, err := RecomputeHash(require, indexes, "19.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Update visits the tree of acme/health-check first, so the dependency
	// reaches acme/base-tools before its own requirement does.
	tree, err := Update(require, indexes, "19.0", nil, "acme/health-check")
	if err != nil {
		t.Fatal(err)
	}
	for _, lf := range []LockFile{full, tree} {
		if bt := lf.Packages["acme/base-tools"]; bt.Version != "19.0.1.0.0" || !bt.Direct {
			t.Errorf("acme/base-tools locked as %+v, want the required 19.0.1.0.0", bt)
		}
	}
}

func TestConflictingRequirements(t *testing.T) {
	indexes, _ := singleIndex(t,
		registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{release("19.0.1.0.0", "acme/base-tools")}},
		registryapi.Addon{Name: "acme/base-tools", Versions: []registryapi.Version{release("19.0.1.0.0"), release("19.0.2.0.0"), release("18.0.1.0.0")}},
	)
	tests := []struct {
		name       string
		constraint string
		want       string
	}{
		{"two exact versions", "19.0.2.0.0", "conflicting requirements for acme/base-tools: the manifest requires 19.0.1.0.0 and resolved 19.0.1.0.0, but acme/other -> acme/health-check requires 19.0.2.0.0"},
		{"another series", "18.0", "but acme/other -> acme/health-check requires 18.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newResolver(map[string]string{"acme/base-tools": "19.0.1.0.0"}, indexes, "19.0", nil)
			err := r.visit([]resolveItem{
				{name: "acme/base-tools", req: requirement{constraint: "19.0.1.0.0"}},
				{name: "acme/base-tools", req: requirement{constraint: tc.constraint, path: []string{"acme/other", "acme/health-check"}}},
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestSatisfies(t *testing.T) {
	tests := []struct {
		constraint, version string
		want                bool
	}{
		{"19.0.1.0.0", "19.0.1.0.0", true},
		{"19.0.1.0.0", "19.0.2.0.0", false},
		{"19.0", "19.0.2.0.0", true},
		{"19.0", "18.0.1.0.0", false},
		{"19.0", "dev-19.0", true},
		{"dev-19.0", "dev-19.0", true},
		{"dev-19.0", "dev-main", false},
		{"dev-19.0", "19.0.1.0.0", false},
		{"19.0.1.0.0", "dev-19.0", false},
	}
	for _, tc := range tests {
		if got := satisfies(tc.constraint, tc.version); got != tc.want {
			t.Errorf("satisfies(%q, %q) = %v, want %v", tc.constraint, tc.version, got, tc.want)
		}
	}
}

func TestRelockKeepsPackageFromImplicitDefault(t *testing.T) {
	indexes, hits := singleIndex(t)
	public, publicHits := registry(t)
	viper.Set("default_index_url", public)

	fromDefault := locked("19.0.1.0.0", true)
	fromDefault.Index = ""
	fromDefault.Registry = public
	old := map[string]LockedPackage{"acme/public-tools": fromDefault}

	lf, err := Relock(map[string]string{"acme/public-tools": "19.0"}, indexes, "19.0", old)
	if err != nil {
		t.Fatal(err)
	}
	if got := lf.Packages["acme/public-tools"]; got.Registry != public || got.Version != "19.0.1.0.0" {
		t.Errorf("locked %+v, want the package kept from the default index", got)
	}
	if n := hits.Load() + publicHits.Load(); n != 0 {
		t.Errorf("relocking a package from the default index sent %d request(s)", n)
	}
}

func TestRelockRefusesModuleCollision(t *testing.T) {
	indexes, _ := singleIndex(t)
	old := map[string]LockedPackage{
		"acme/health-check": locked("19.0.1.0.0", true),
		"acme/health_check": locked("19.0.1.0.0", true),
	}
	_, err := Relock(map[string]string{"acme/health-check": "19.0", "acme/health_check": "19.0"}, indexes, "19.0", old)
	if err == nil || !strings.Contains(err.Error(), "acme_health_check") {
		t.Fatalf("got error %v, want one naming the shared addon directory", err)
	}
}

func TestRenamedIndexKeepsLockfile(t *testing.T) {
	served := registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{release("19.0.1.0.0")}}
	indexes, hits := singleIndex(t, served)
	require := map[string]string{"acme/health-check": "19.0"}

	lf, err := RecomputeHash(require, indexes, "19.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	renamed := manifest.Indexes{"company": {Url: indexes["a"].Url + "/", Type: "registry"}}
	if stale, err := IsStale(require, renamed, "19.0", lf.Packages, lf.ContentHash); err != nil || stale {
		t.Fatalf("renaming the index made the lockfile stale (%v, %v)", stale, err)
	}

	hits.Store(0)
	again, err := Relock(require, renamed, "19.0", lf.Packages)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, lf) {
		t.Errorf("relocking after the rename changed the lockfile:\n%+v\nwant\n%+v", again, lf)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("relocking after the rename sent %d request(s)", n)
	}
}

func TestLockfileWithIndexNameIsWrittenBackWithRegistry(t *testing.T) {
	indexes, hits := singleIndex(t)
	require := map[string]string{"acme/health-check": "19.0"}
	old := map[string]LockedPackage{"acme/health-check": locked("19.0.1.0.0", true)}

	hash, err := namedHash(require, indexes, "19.0", old)
	if err != nil {
		t.Fatal(err)
	}
	if stale, err := IsStale(require, indexes, "19.0", old, hash); err != nil || stale {
		t.Fatalf("a lockfile hashed with index names is stale (%v, %v)", stale, err)
	}

	lf, err := Relock(require, indexes, "19.0", old)
	if err != nil {
		t.Fatal(err)
	}
	if got := lf.Packages["acme/health-check"]; got.Registry != indexes["a"].Url || got.Index != "" || got.Version != "19.0.1.0.0" {
		t.Errorf("locked %+v, want the package kept with the URL of index a (%s)", got, indexes["a"].Url)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("rewriting the index name sent %d request(s)", n)
	}
}

func TestChangedRegistryURLResolvesAgain(t *testing.T) {
	served := registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{release("19.0.1.0.0")}}
	indexes, hits := singleIndex(t, served)
	moved := locked("19.0.1.0.0", true)
	moved.Index = ""
	moved.Registry = "https://old.example.com"

	lf, err := Relock(map[string]string{"acme/health-check": "19.0"}, indexes, "19.0", map[string]LockedPackage{"acme/health-check": moved})
	if err != nil {
		t.Fatal(err)
	}
	if got := lf.Packages["acme/health-check"].Registry; got != indexes["a"].Url {
		t.Errorf("locked to %q, want index a (%s)", got, indexes["a"].Url)
	}
	if hits.Load() == 0 {
		t.Error("a package locked to a registry no index points at was kept without asking index a")
	}
}
