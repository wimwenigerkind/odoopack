package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRequestRefusesTokenOverPlainHTTP(t *testing.T) {
	_, err := NewRequest(context.Background(), "http://registry.example.com/registry/v1/addons/acme/health-check", "secret")
	if err == nil || !strings.Contains(err.Error(), "http://registry.example.com/registry/v1/addons/acme/health-check") {
		t.Fatalf("got error %v, want one naming the url", err)
	}

	if _, err := NewRequest(context.Background(), "http://registry.example.com/", ""); err != nil {
		t.Fatalf("request without a token: %v", err)
	}
}

func TestNewRequestSendsTokenOverHTTPToLocalhost(t *testing.T) {
	for _, url := range []string{
		"http://localhost:8080/",
		"http://127.0.0.1:8080/",
		"http://[::1]:8080/",
		"https://registry.example.com/",
	} {
		req, err := NewRequest(context.Background(), url, "secret")
		if err != nil {
			t.Errorf("%s: %v", url, err)
			continue
		}
		if got := req.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("%s: got Authorization %q", url, got)
		}
	}
}

// recordAuth serves 200 and records the Authorization header it got.
func recordAuth(t *testing.T, srv func(http.Handler) *httptest.Server) (*httptest.Server, *atomic.Value) {
	t.Helper()
	var got atomic.Value
	s := srv(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("Authorization"))
	}))
	t.Cleanup(s.Close)
	return s, &got
}

func redirectTo(t *testing.T, srv func(http.Handler) *httptest.Server, target string) *httptest.Server {
	t.Helper()
	s := srv(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}))
	t.Cleanup(s.Close)
	return s
}

func get(t *testing.T, client *http.Client, url string) error {
	t.Helper()
	req, err := NewRequest(context.Background(), url, "secret")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

// Go keeps Authorization on a redirect to another port of the same host.
func TestRedirectToOtherPortDropsToken(t *testing.T) {
	target, auth := recordAuth(t, httptest.NewServer)
	origin := redirectTo(t, httptest.NewServer, target.URL)

	if err := get(t, Metadata, origin.URL); err != nil {
		t.Fatal(err)
	}
	if got := auth.Load(); got != "" {
		t.Fatalf("redirect target got Authorization %q", got)
	}
}

func TestRedirectToSameOriginKeepsToken(t *testing.T) {
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusFound)
			return
		}
		got.Store(r.Header.Get("Authorization"))
	}))
	t.Cleanup(srv.Close)

	if err := get(t, Metadata, srv.URL+"/old"); err != nil {
		t.Fatal(err)
	}
	if got.Load() != "Bearer secret" {
		t.Fatalf("redirect target got Authorization %q", got.Load())
	}
}

func TestRedirectFromHTTPSToHTTPIsRefused(t *testing.T) {
	target, auth := recordAuth(t, httptest.NewServer)
	origin := redirectTo(t, httptest.NewTLSServer, target.URL)

	client := newClient(time.Minute, time.Minute)
	client.Transport.(*http.Transport).TLSClientConfig = origin.Client().Transport.(*http.Transport).TLSClientConfig

	err := get(t, client, origin.URL)
	if err == nil || !strings.Contains(err.Error(), "leaves https") {
		t.Fatalf("got error %v, want the redirect refused", err)
	}
	if auth.Load() != nil {
		t.Fatal("the http target was requested")
	}
}

func hanging(_ http.ResponseWriter, r *http.Request) {
	<-r.Context().Done()
}

func TestHangingServerHitsHeaderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(hanging))
	t.Cleanup(srv.Close)

	err := get(t, newClient(0, 100*time.Millisecond), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "timeout awaiting response headers") {
		t.Fatalf("got error %v, want a header timeout", err)
	}
}

func TestDownloadStallsAfterIdleTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	err := fetch(newClient(0, time.Minute), 100*time.Millisecond, &buf, srv.URL, "", 1<<20)
	if err == nil || !strings.Contains(err.Error(), "no data received for 100ms") {
		t.Fatalf("got error %v, want a stall", err)
	}
}

func TestDownloadKeepsGoingWhileBytesArrive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for range 5 {
			_, _ = w.Write([]byte("x"))
			w.(http.Flusher).Flush()
			time.Sleep(40 * time.Millisecond)
		}
	}))
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	if err := fetch(newClient(0, time.Minute), 100*time.Millisecond, &buf, srv.URL, "", 1<<20); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "xxxxx" {
		t.Fatalf("got body %q", buf.String())
	}
}

func TestDownloadRefusesOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 11))
	}))
	t.Cleanup(srv.Close)

	if err := Download(io.Discard, srv.URL, "", 10); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got error %v, want ErrTooLarge", err)
	}
	if err := Download(io.Discard, srv.URL, "", 11); err != nil {
		t.Fatalf("body at the limit: %v", err)
	}
}

func TestReadAllRefusesOversizedBody(t *testing.T) {
	if _, err := ReadAll(strings.NewReader("12345678901"), 10); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got error %v, want ErrTooLarge", err)
	}
	if body, err := ReadAll(strings.NewReader("1234567890"), 10); err != nil || len(body) != 10 {
		t.Fatalf("body at the limit: %q, %v", body, err)
	}
}
