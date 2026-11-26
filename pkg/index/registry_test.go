package index

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"github.com/wimwenigerkind/odoopack/pkg/buildinfo"
	"github.com/wimwenigerkind/odoopack/pkg/httpclient"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

const testDigest = "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543"

func validVersion(v string) registryapi.Version {
	return registryapi.Version{
		Version: v,
		Type:    registryapi.TypeZip,
		URL:     "https://registry.example.com/registry/v1/zipball/f1740dd9-ca0e-42fd-a63b-b553d3fd3882/ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543",
		Digest:  testDigest,
	}
}

// serveAddon answers every request with addon and records the escaped path
// of the last request.
func serveAddon(t *testing.T, addon registryapi.Addon) (*RegistryProvider, *atomic.Value) {
	t.Helper()
	var path atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path.Store(r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(addon)
	}))
	t.Cleanup(srv.Close)
	return &RegistryProvider{BaseURL: srv.URL}, &path
}

// captureStdout returns what fn prints to stdout, where ui writes warnings.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	return <-done
}

func TestRegistryLookupRequestsVendorNameUnescaped(t *testing.T) {
	p, path := serveAddon(t, registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{validVersion("19.0.1.0.0")}})

	got, err := p.Lookup("acme/health-check", "19.0", "19.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "19.0.1.0.0" {
		t.Fatalf("resolved %s, want 19.0.1.0.0", got.Version)
	}
	if want := "/registry/v1/addons/acme/health-check"; path.Load() != want {
		t.Fatalf("requested %v, want %s", path.Load(), want)
	}
}

func TestRegistryLookupRejectsInvalidRequestedName(t *testing.T) {
	p, path := serveAddon(t, registryapi.Addon{Name: "acme/health-check"})

	for _, name := range []string{"", "..", "acme/../x", "acme/health-check/extra", "Acme/health-check"} {
		if _, err := p.Lookup(name, "19.0", "19.0"); err == nil {
			t.Errorf("Lookup(%q) succeeded", name)
		}
	}
	if path.Load() != nil {
		t.Fatalf("an invalid name reached the registry as %v", path.Load())
	}
}

func TestRegistryLookupRejectsInvalidResponseName(t *testing.T) {
	for _, name := range []string{"", "..", "../acme"} {
		p, _ := serveAddon(t, registryapi.Addon{Name: name, Versions: []registryapi.Version{validVersion("19.0.1.0.0")}})
		_, err := p.Lookup("acme/health-check", "19.0", "19.0")
		if err == nil || !strings.Contains(err.Error(), "invalid name") {
			t.Errorf("response name %q: got error %v, want an invalid name", name, err)
		}
	}
}

func TestRegistryLookupRejectsOtherResponseName(t *testing.T) {
	p, _ := serveAddon(t, registryapi.Addon{Name: "acme/other", Versions: []registryapi.Version{validVersion("19.0.1.0.0")}})

	_, err := p.Lookup("acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), `with addon "acme/other"`) {
		t.Fatalf("got error %v, want a name mismatch", err)
	}
}

func TestRegistryLookupSkipsInvalidVersion(t *testing.T) {
	broken := validVersion("19.0.2.0.0")
	broken.Digest = ""
	p, _ := serveAddon(t, registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{validVersion("19.0.1.0.0"), broken}})

	var got AddonVersion
	var err error
	out := captureStdout(t, func() {
		got, err = p.Lookup("acme/health-check", "19.0", "19.0")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "19.0.1.0.0" {
		t.Fatalf("resolved %s, want the valid 19.0.1.0.0", got.Version)
	}
	if n := strings.Count(out, "skipping"); n != 1 || !strings.Contains(out, "19.0.2.0.0") {
		t.Fatalf("want one warning about 19.0.2.0.0, got %q", out)
	}
}

func TestRegistryLookupReportsProblemDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"type":"about:blank","title":"Internal Server Error","status":500,"code":"internal_error","detail":"database unavailable"}`)
	}))
	t.Cleanup(srv.Close)

	_, err := (&RegistryProvider{BaseURL: srv.URL}).Lookup("acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("got error %v, want the problem detail", err)
	}
}

func TestRegistryLookupSendsUserAgent(t *testing.T) {
	version := buildinfo.Version
	buildinfo.Version = "1.2.3"
	t.Cleanup(func() { buildinfo.Version = version })

	var userAgent atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent.Store(r.UserAgent())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{validVersion("19.0.1.0.0")}})
	}))
	t.Cleanup(srv.Close)

	if _, err := (&RegistryProvider{BaseURL: srv.URL}).Lookup("acme/health-check", "19.0", "19.0"); err != nil {
		t.Fatal(err)
	}
	if got := userAgent.Load(); got != "odoopack/1.2.3" {
		t.Fatalf("got User-Agent %v, want odoopack/1.2.3", got)
	}
}

func TestRegistryLookupRefusesOversizedMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"acme/health-check","description":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", maxMetadataBytes)))
		_, _ = w.Write([]byte(`"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := (&RegistryProvider{BaseURL: srv.URL}).Lookup("acme/health-check", "19.0", "19.0")
	if !errors.Is(err, httpclient.ErrTooLarge) || !strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("got error %v, want ErrTooLarge naming the registry", err)
	}
}

func TestLookupRefusesTokenOverPlainHTTP(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Set("auth", `{"bearer":{"registry.example.com":"secret"}}`)

	indexes := manifest.Indexes{{Name: "company", Url: "http://registry.example.com", Type: "registry"}}
	_, err := Lookup(indexes, "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "index company (http://registry.example.com)") || !strings.Contains(err.Error(), "plain http") {
		t.Fatalf("got error %v, want the token refused for index company", err)
	}
}

func TestLookupSendsTokenOverHTTPToLocalhost(t *testing.T) {
	var auth atomic.Value
	url, _ := registry(t, func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		serving(w, r)
	})
	t.Cleanup(viper.Reset)
	viper.Set("auth", `{"bearer":{"`+strings.TrimPrefix(url, "http://")+`":"secret"}}`)

	if _, err := Lookup(manifest.Indexes{{Name: "local", Url: url, Type: "registry"}}, "acme/health-check", "19.0", "19.0"); err != nil {
		t.Fatal(err)
	}
	if got := auth.Load(); got != "Bearer secret" {
		t.Fatalf("got Authorization %v", got)
	}
}

func TestLookupFailsOnMalformedAuth(t *testing.T) {
	url, hits := registry(t, serving)
	t.Cleanup(viper.Reset)
	viper.Set("auth", `registry.example.com=secret`)

	_, err := Lookup(manifest.Indexes{{Name: "local", Url: url, Type: "registry"}}, "acme/health-check", "19.0", "19.0")
	if err == nil || !strings.Contains(err.Error(), "ODOOPACK_AUTH is not valid") {
		t.Fatalf("got error %v, want ODOOPACK_AUTH refused", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the registry was requested")
	}
}
