package index

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/wimwenigerkind/odoopack/pkg/auth"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

type AddonVersion struct {
	Name       string
	Version    string
	Type       string
	Repository string
	Reference  string
	Shasum     string
	Depends    []Dep
	// Registry is the normalized URL of the index the addon came from. Lookup
	// sets it, a Provider does not know it.
	Registry string
}

type Dep struct {
	Module  string
	Package string
	Access  string
}

type Provider interface {
	Lookup(name, constraint, odooSeries string) (AddonVersion, error)
}

func NewProvider(repoType, url, token string) (Provider, error) {
	switch repoType {
	case "registry":
		return &RegistryProvider{BaseURL: url, Token: token}, nil
	default:
		return nil, fmt.Errorf("unknown repository type %q", repoType)
	}
}

// ErrNotFound marks an addon an index does not have. It is the only error
// that lets resolution go on to the next index: any other one, such as an
// expired token or an unavailable registry, could otherwise install a
// public addon in place of a private one with the same name.
var ErrNotFound = errors.New("not found")

// Lookup resolves an addon that is not locked to an index yet.
func Lookup(indexes manifest.Indexes, name, constraint, odooSeries string) (AddonVersion, error) {
	return LookupLocked(indexes, "", name, constraint, odooSeries)
}

// NormalizeURL is the form a registry URL is locked under: scheme and host
// in lower case, no trailing slash, so that two spellings of one index lock
// the same.
func NormalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String()
}

// locatedBy reports whether locked points at idx. A package is locked to
// the URL of its index, so renaming the index does not change the lockfile.
// Lockfiles written before that hold the name of the index instead.
func locatedBy(idx manifest.Index, locked string) bool {
	return locked != "" && (NormalizeURL(idx.Url) == locked || idx.Name == locked)
}

// LockedRegistry returns the normalized URL of the index LookupLocked asks
// for a package locked to locked, and false when no index searched matches.
func LockedRegistry(indexes manifest.Indexes, locked string) (string, bool) {
	for _, idx := range indexes {
		if locatedBy(idx, locked) {
			return NormalizeURL(idx.Url), true
		}
	}
	return "", false
}

// LookupLocked resolves an addon from the index locked points at only, when
// that index still exists. locked is the URL of the index or, in a lockfile
// written before URLs were locked, its name. Otherwise it tries the indexes
// of the manifest in their order.
func LookupLocked(indexes manifest.Indexes, locked, name, constraint, odooSeries string) (AddonVersion, error) {
	for _, idx := range indexes {
		if !locatedBy(idx, locked) {
			continue
		}
		result, err := lookupIn(idx, name, constraint, odooSeries)
		if err != nil {
			return AddonVersion{}, fmt.Errorf("addon %q@%s is locked to index %s (%s): %w", name, constraint, idx.Name, idx.Url, err)
		}
		result.Registry = NormalizeURL(idx.Url)
		return result, nil
	}

	var attempts []string
	for _, idx := range indexes {
		result, err := lookupIn(idx, name, constraint, odooSeries)
		if err == nil {
			result.Registry = NormalizeURL(idx.Url)
			return result, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return AddonVersion{}, fmt.Errorf("addon %q@%s: index %s (%s): %w", name, constraint, idx.Name, idx.Url, err)
		}
		attempts = append(attempts, fmt.Sprintf("%s[%s]: %v", idx.Name, idx.Url, err))
	}

	if len(attempts) == 0 {
		return AddonVersion{}, fmt.Errorf("addon %q@%s: no indexes configured", name, constraint)
	}
	return AddonVersion{}, fmt.Errorf("addon %q@%s not found:\n  %s", name, constraint, strings.Join(attempts, "\n  "))
}

func lookupIn(idx manifest.Index, name, constraint, odooSeries string) (AddonVersion, error) {
	token, err := auth.TokenForURL(idx.Url)
	if err != nil {
		return AddonVersion{}, err
	}
	provider, err := NewProvider(idx.Type, idx.Url, token)
	if err != nil {
		return AddonVersion{}, err
	}
	return provider.Lookup(name, constraint, odooSeries)
}
