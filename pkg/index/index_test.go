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
			indexes := manifest.Indexes{
				{Name: "a", Url: first, Type: "registry"},
				{Name: "b", Url: second, Type: "registry"},
			}

			got, err := Lookup(indexes, "acme/health-check", "19.0", "19.0")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if got.Registry != second {
					t.Fatalf("resolved from %q, want index b (%s)", got.Registry, second)
				}
				return
			}
			if err == nil {
				t.Fatalf("resolved from %q, want an error", got.Registry)
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
	indexes := manifest.Indexes{
		{Name: "a", Url: first, Type: "registry"},
		{Name: "b", Url: second, Type: "registry"},
	}

	_, err := Lookup(indexes, "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "a["+first+"]") || !strings.Contains(err.Error(), "b["+second+"]") {
		t.Fatalf("got error %v, want one listing both indexes", err)
	}
	if strings.Contains(err.Error(), "default index") {
		t.Fatalf("got error %v, which mentions a default index although none is set", err)
	}
}

func TestLookupFollowsIndexOrder(t *testing.T) {
	company, companyHits := registry(t, serving)
	public, publicHits := registry(t, serving)

	got, err := Lookup(manifest.Indexes{
		{Name: "public", Url: public, Type: "registry"},
		{Name: "company", Url: company, Type: "registry"},
	}, "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Registry != public || companyHits.Load() != 0 {
		t.Fatalf("resolved from %q with %d request(s) to company, want the first index %s only", got.Registry, companyHits.Load(), public)
	}

	publicHits.Store(0)
	got, err = Lookup(manifest.Indexes{
		{Name: "company", Url: company, Type: "registry"},
		{Name: "public", Url: public, Type: "registry"},
	}, "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Registry != company || publicHits.Load() != 0 {
		t.Fatalf("resolved from %q with %d request(s) to public, want the first index %s only", got.Registry, publicHits.Load(), company)
	}
}

func TestLookupNeverFallsBackOnErrors(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		configured, _ := registry(t, status(code))
		public, publicHits := registry(t, serving)

		_, err := Lookup(manifest.Indexes{
			{Name: "company", Url: configured, Type: "registry"},
			{Name: "public", Url: public, Type: "registry"},
		}, "acme/health-check", "19.0", "19.0")
		if err == nil || !strings.Contains(err.Error(), "index company") {
			t.Fatalf("status %d: got error %v, want one naming the configured index", code, err)
		}
		if n := publicHits.Load(); n != 0 {
			t.Fatalf("status %d: the next index got %d request(s)", code, n)
		}
	}
}

func TestLookupHasNoImplicitDefault(t *testing.T) {
	public, hits := registry(t, serving)
	viper.Set("default_index_url", public)
	t.Cleanup(viper.Reset)

	_, err := Lookup(nil, "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "no indexes configured") {
		t.Fatalf("got error %v, want one about no indexes", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the default registry got %d request(s) although no index lists it", n)
	}
}

func TestLookupLockedAsksOnlyItsIndex(t *testing.T) {
	first, firstHits := registry(t, serving)
	second, _ := registry(t, serving)
	indexes := manifest.Indexes{
		{Name: "a", Url: first, Type: "registry"},
		{Name: "b", Url: second, Type: "registry"},
	}

	got, err := LookupLocked(indexes, second, "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Registry != second || firstHits.Load() != 0 {
		t.Fatalf("resolved from %q with %d request(s) to a, want b only", got.Registry, firstHits.Load())
	}
}

func TestLookupLockedFindsRenamedIndexByURL(t *testing.T) {
	first, firstHits := registry(t, serving)
	second, _ := registry(t, serving)
	indexes := manifest.Indexes{
		{Name: "a", Url: first, Type: "registry"},
		{Name: "renamed", Url: second, Type: "registry"},
	}

	got, err := LookupLocked(indexes, second, "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Registry != second || firstHits.Load() != 0 {
		t.Fatalf("resolved from %q with %d request(s) to a, want the renamed index only", got.Registry, firstHits.Load())
	}
}

func TestLookupLockedDoesNotFallThrough(t *testing.T) {
	first, _ := registry(t, serving)
	second, _ := registry(t, status(http.StatusNotFound))
	indexes := manifest.Indexes{
		{Name: "a", Url: first, Type: "registry"},
		{Name: "b", Url: second, Type: "registry"},
	}

	_, err := LookupLocked(indexes, second, "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "locked to index b") {
		t.Fatalf("got error %v, want one naming the locked index b", err)
	}
}

func TestLookupLockedToRemovedIndexResolvesAgain(t *testing.T) {
	first, _ := registry(t, serving)

	got, err := LookupLocked(manifest.Indexes{{Name: "a", Url: first, Type: "registry"}}, "https://gone.example.com", "acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Registry != first {
		t.Fatalf("resolved from %q, want index a (%s)", got.Registry, first)
	}
}
