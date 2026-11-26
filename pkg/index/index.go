package index

import (
	"errors"
	"fmt"
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
	Digest     string
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

// locatedBy reports whether a package locked to the index URL locked comes
// from idx. A package is locked to the URL, so renaming the index does not
// change the lockfile.
func locatedBy(idx manifest.Index, locked string) bool {
	return locked != "" && idx.Url == locked
}

// HasIndex reports whether indexes still have the index a package is locked
// to by its URL.
func HasIndex(indexes manifest.Indexes, locked string) bool {
	for _, idx := range indexes {
		if locatedBy(idx, locked) {
			return true
		}
	}
	return false
}

// LookupLocked resolves an addon from the index with the URL locked only,
// when that index still exists. Otherwise it tries the indexes of the
// manifest in their order.
func LookupLocked(indexes manifest.Indexes, locked, name, constraint, odooSeries string) (AddonVersion, error) {
	for _, idx := range indexes {
		if !locatedBy(idx, locked) {
			continue
		}
		result, err := lookupIn(idx, name, constraint, odooSeries)
		if err != nil {
			return AddonVersion{}, fmt.Errorf("addon %q@%s is locked to index %s (%s): %w", name, constraint, idx.Name, idx.Url, err)
		}
		result.Registry = idx.Url
		return result, nil
	}

	var attempts []string
	for _, idx := range indexes {
		result, err := lookupIn(idx, name, constraint, odooSeries)
		if err == nil {
			result.Registry = idx.Url
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
