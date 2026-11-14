package lockfile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	if got := lf.Packages["acme/health-check"].Index; got != "a" {
		t.Errorf("acme/health-check locked to index %q, want a", got)
	}
	if got := lf.Packages["acme/base-tools"].Index; got != "b" {
		t.Errorf("acme/base-tools locked to index %q, want b", got)
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
	if got := lf.Packages["acme/health-check"].Index; got != "default" {
		t.Fatalf("locked to index %q, want default", got)
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
		if pkg.Index != "b" {
			t.Errorf("%s locked to index %q, want b", name, pkg.Index)
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
	if got := lf.Packages["acme/health-check"].Index; got != "b" {
		t.Fatalf("locked to index %q, want b", got)
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
	path := useLock(t, `{"format":1,"content_hash":"`+hash+`","packages":{"acme/health-check":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"},"direct":true}}}`)

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
