// Package httpclient holds the one HTTP setup that registry metadata and
// zipball downloads share: timeouts, size limits and the rules for when a
// bearer token may be sent.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/wimwenigerkind/odoopack/pkg/buildinfo"
)

const (
	dialTimeout           = 10 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 30 * time.Second
	// metadataTimeout bounds a whole metadata request including its body.
	metadataTimeout = 60 * time.Second
	// A zipball can be large on a slow link, so a download has no overall
	// timeout and is cancelled only when no bytes arrive for this long.
	downloadIdleTimeout = 60 * time.Second
	maxRedirects        = 10
)

// ErrTooLarge marks a response body above the limit its caller set.
var ErrTooLarge = errors.New("response exceeds the size limit")

// Metadata is the client for small JSON requests to a registry.
var Metadata = newClient(metadataTimeout, responseHeaderTimeout)

var download = newClient(0, responseHeaderTimeout)

func newClient(timeout, headerTimeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = tlsHandshakeTimeout
	transport.ResponseHeaderTimeout = headerTimeout
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: checkRedirect}
}

// checkRedirect keeps the token with the origin it was configured for. Go
// itself only drops Authorization when the host changes and keeps it for
// another port or scheme on the same host.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if prev := via[len(via)-1].URL; prev.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect from %s to %s: it leaves https", prev.Redacted(), req.URL.Redacted())
	}
	if !sameOrigin(via[0].URL, req.URL) {
		req.Header.Del("Authorization")
	}
	return nil
}

func sameOrigin(a, b *url.URL) bool {
	return a.Scheme == b.Scheme && a.Hostname() == b.Hostname() && port(a) == port(b)
}

func port(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// NewRequest builds a GET for rawURL with the odoopack User-Agent and, when
// token is set, the bearer token. A token only travels over https, or over
// plain http to the local machine for a registry under development; anything
// else fails so that a misconfigured index is noticed rather than leaking the
// token or silently dropping it.
func NewRequest(ctx context.Context, rawURL, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	if token == "" {
		return req, nil
	}
	if req.URL.Scheme != "https" && !isLoopback(req.URL.Hostname()) {
		return nil, fmt.Errorf("refusing to send the ODOOPACK_AUTH token for %s over %s: use https (plain http is only allowed for localhost)", req.URL.Redacted(), req.URL.Scheme)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ReadAll reads r up to limit bytes and fails with ErrTooLarge beyond that.
func ReadAll(r io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w of %d bytes", ErrTooLarge, limit)
	}
	return body, nil
}

// Download writes the body of a GET for rawURL to w and fails with
// ErrTooLarge once it exceeds limit bytes. On an error w may hold part of the
// body.
func Download(w io.Writer, rawURL, token string, limit int64) error {
	return fetch(download, downloadIdleTimeout, w, rawURL, token, limit)
}

func fetch(client *http.Client, idle time.Duration, w io.Writer, rawURL, token string, limit int64) error {
	stalled := fmt.Errorf("download %s: no data received for %s", rawURL, idle)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	timer := time.AfterFunc(idle, func() { cancel(stalled) })
	defer timer.Stop()

	req, err := NewRequest(ctx, rawURL, token)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(context.Cause(ctx), stalled) {
			return stalled
		}
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: bad status %s", rawURL, response.Status)
	}

	body := &idleReader{r: io.LimitReader(response.Body, limit+1), timer: timer, idle: idle}
	n, err := io.Copy(w, body)
	if err != nil {
		if errors.Is(context.Cause(ctx), stalled) {
			return stalled
		}
		return fmt.Errorf("download %s: %w", rawURL, err)
	}
	if n > limit {
		return fmt.Errorf("download %s: %w of %d bytes", rawURL, ErrTooLarge, limit)
	}
	return nil
}

// idleReader pushes the idle deadline back whenever bytes arrive.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	idle  time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.timer.Reset(r.idle)
	}
	return n, err
}
