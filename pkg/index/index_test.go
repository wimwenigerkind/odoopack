package index

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

// registry serves handler and counts the requests it gets.
func registry(t *testing.T, handler http.HandlerFunc) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

func serving(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{validVersion("19.0.1.0.0")}})
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func hanging(_ http.ResponseWriter, r *http.Request) {
	<-r.Context().Done()
}

// useDefault sets the implicit default index for the test.
func useDefault(t *testing.T, url string) {
	t.Helper()
	viper.Set("default_index_url", url)
	t.Cleanup(viper.Reset)
}

func TestLookupFallsThroughOnlyOnNotFound(t *testing.T) {
	client := httpClient
	httpClient = &http.Client{Timeout: 200 * time.Millisecond}
	t.Cleanup(func() { httpClient = client })

	for _, tc := range []struct {
		name    string
		first   http.HandlerFunc
		wantErr string
	}{
		{"401", status(http.StatusUnauthorized), "authentication required"},
		{"403", status(http.StatusForbidden), "access denied"},
		{"500", status(http.StatusInternalServerError), "500 Internal Server Error"},
		{"timeout", hanging, "Client.Timeout"},
		{"404", status(http.StatusNotFound), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, _ := registry(t, tc.first)
			second, secondHits := registry(t, serving)
			useDefault(t, "")
			indexes := manifest.Indexes{
				"a": {Url: first, Type: "registry"},
				"b": {Url: second, Type: "registry"},
			}

			got, err := Lookup(indexes, "acme/health-check", "19.0", "19.0")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if got.Index != "b" {
					t.Fatalf("resolved from index %q, want b", got.Index)
				}
				return
			}
			if err == nil {
				t.Fatalf("resolved from index %q, want an error", got.Index)
			}
			if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "index a ("+first+")") {
				t.Fatalf("got error %v, want one naming index a and containing %q", err, tc.wantErr)
			}
			if errors.Is(err, ErrNotFound) {
				t.Fatalf("error %v is ErrNotFound", err)
			}
			if n := secondHits.Load(); n != 0 {
				t.Fatalf("index b got %d request(s) after index a failed", n)
			}
		})
	}
}

func TestLookupNotFoundListsAttempts(t *testing.T) {
	first, _ := registry(t, status(http.StatusNotFound))
	second, _ := registry(t, status(http.StatusNotFound))
	useDefault(t, "")
	indexes := manifest.Indexes{
		"a": {Url: first, Type: "registry"},
		"b": {Url: second, Type: "registry"},
	}

	_, err := Lookup(indexes, "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "a["+first+"]") || !strings.Contains(err.Error(), "b["+second+"]") {
		t.Fatalf("got error %v, want one listing both indexes", err)
	}
	if strings.Contains(err.Error(), "default index") {
		t.Fatalf("got error %v, which mentions a default index although none is set", err)
	}
}

func TestLookupTriesImplicitDefaultLast(t *testing.T) {
	configured, configuredHits := registry(t, status(http.StatusNotFound))
	public, publicHits := registry(t, serving)
	useDefault(t, public)

	got, err := Lookup(manifest.Indexes{"company": {Url: configured, Type: "registry"}}, "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != DefaultIndexName {
		t.Fatalf("resolved from index %q, want %s", got.Index, DefaultIndexName)
	}
	if configuredHits.Load() != 1 || publicHits.Load() != 1 {
		t.Fatalf("got %d request(s) to the configured and %d to the default index, want one each", configuredHits.Load(), publicHits.Load())
	}
}

func TestLookupNeverFallsBackToDefaultOnErrors(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		configured, _ := registry(t, status(code))
		public, publicHits := registry(t, serving)
		useDefault(t, public)

		_, err := Lookup(manifest.Indexes{"company": {Url: configured, Type: "registry"}}, "acme/health-check", "19.0", "19.0")
		if err == nil || !strings.Contains(err.Error(), "index company") {
			t.Fatalf("status %d: got error %v, want one naming the configured index", code, err)
		}
		if n := publicHits.Load(); n != 0 {
			t.Fatalf("status %d: the default index got %d request(s)", code, n)
		}
	}
}

func TestLookupSkipsImplicitDefaultConfiguredExplicitly(t *testing.T) {
	public, publicHits := registry(t, status(http.StatusNotFound))
	useDefault(t, public)

	for _, indexes := range []manifest.Indexes{
		{"public": {Url: public, Type: "registry"}},
		{"default": {Url: public, Type: "registry"}},
	} {
		publicHits.Store(0)
		if _, err := Lookup(indexes, "acme/health-check", "19.0", "19.0"); err == nil {
			t.Fatal("expected not found")
		}
		if n := publicHits.Load(); n != 1 {
			t.Fatalf("indexes %v: the default registry got %d request(s), want exactly one", indexes, n)
		}
	}
}

func TestLookupUsesImplicitDefaultWithoutIndexes(t *testing.T) {
	public, _ := registry(t, serving)
	useDefault(t, public)

	got, err := Lookup(nil, "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != DefaultIndexName {
		t.Fatalf("resolved from index %q, want %s", got.Index, DefaultIndexName)
	}
}

func TestLookupLockedAsksOnlyItsIndex(t *testing.T) {
	first, firstHits := registry(t, serving)
	second, _ := registry(t, serving)
	useDefault(t, "")
	indexes := manifest.Indexes{
		"a": {Url: first, Type: "registry"},
		"b": {Url: second, Type: "registry"},
	}

	got, err := LookupLocked(indexes, "b", "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != "b" || firstHits.Load() != 0 {
		t.Fatalf("resolved from index %q with %d request(s) to a, want b only", got.Index, firstHits.Load())
	}
}

func TestLookupLockedDoesNotFallThrough(t *testing.T) {
	first, _ := registry(t, serving)
	second, _ := registry(t, status(http.StatusNotFound))
	useDefault(t, "")
	indexes := manifest.Indexes{
		"a": {Url: first, Type: "registry"},
		"b": {Url: second, Type: "registry"},
	}

	_, err := LookupLocked(indexes, "b", "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "locked to index b") {
		t.Fatalf("got error %v, want one naming the locked index b", err)
	}
}

func TestLookupLockedToRemovedIndexResolvesAgain(t *testing.T) {
	first, _ := registry(t, serving)
	useDefault(t, "")

	got, err := LookupLocked(manifest.Indexes{"a": {Url: first, Type: "registry"}}, "gone", "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != "a" {
		t.Fatalf("resolved from index %q, want a", got.Index)
	}
}
