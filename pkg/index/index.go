package index

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/viper"
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
	// Index names the index the addon came from. Lookup sets it, a Provider
	// does not know it.
	Index string
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

// DefaultIndexName is the name the implicit default index is recorded under.
const DefaultIndexName = "default"

// Lookup resolves an addon that is not locked to an index yet.
func Lookup(indexes manifest.Indexes, name, constraint, odooSeries string) (AddonVersion, error) {
	return LookupLocked(indexes, "", name, constraint, odooSeries)
}

type namedIndex struct {
	name  string
	index manifest.Index
}

// searchOrder lists the indexes of the manifest in alphabetical order and the
// implicit default index last.
func searchOrder(indexes manifest.Indexes) []namedIndex {
	keys := make([]string, 0, len(indexes))
	for k := range indexes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	order := make([]namedIndex, 0, len(keys)+1)
	for _, k := range keys {
		order = append(order, namedIndex{k, indexes[k]})
	}
	if defaultURL := viper.GetString("default_index_url"); UsesImplicitDefault(indexes, defaultURL) {
		order = append(order, namedIndex{DefaultIndexName, manifest.Index{Url: defaultURL, Type: "registry"}})
	}
	return order
}

// UsesImplicitDefault reports whether the built-in default index is searched
// after the indexes of the manifest: when it is set and the manifest neither
// names an index default nor points one at the same URL.
func UsesImplicitDefault(indexes manifest.Indexes, defaultURL string) bool {
	if defaultURL == "" {
		return false
	}
	if _, ok := indexes[DefaultIndexName]; ok {
		return false
	}
	for _, idx := range indexes {
		if idx.Url == defaultURL {
			return false
		}
	}
	return true
}

// LookupLocked resolves an addon from lockedIndex only, when that index still
// exists. Otherwise it tries the indexes of the manifest in alphabetical order
// and the implicit default index last.
func LookupLocked(indexes manifest.Indexes, lockedIndex, name, constraint, odooSeries string) (AddonVersion, error) {
	order := searchOrder(indexes)

	for _, ni := range order {
		if lockedIndex == "" || ni.name != lockedIndex {
			continue
		}
		result, err := lookupIn(ni.index, name, constraint, odooSeries)
		if err != nil {
			return AddonVersion{}, fmt.Errorf("addon %q@%s is locked to index %s (%s): %w", name, constraint, ni.name, ni.index.Url, err)
		}
		result.Index = ni.name
		return result, nil
	}

	var attempts []string
	for _, ni := range order {
		result, err := lookupIn(ni.index, name, constraint, odooSeries)
		if err == nil {
			result.Index = ni.name
			return result, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return AddonVersion{}, fmt.Errorf("addon %q@%s: index %s (%s): %w", name, constraint, ni.name, ni.index.Url, err)
		}
		attempts = append(attempts, fmt.Sprintf("%s[%s]: %v", ni.name, ni.index.Url, err))
	}

	if len(attempts) == 0 {
		return AddonVersion{}, fmt.Errorf("addon %q@%s: no indexes configured", name, constraint)
	}
	return AddonVersion{}, fmt.Errorf("addon %q@%s not found:\n  %s", name, constraint, strings.Join(attempts, "\n  "))
}

func lookupIn(idx manifest.Index, name, constraint, odooSeries string) (AddonVersion, error) {
	provider, err := NewProvider(idx.Type, idx.Url, auth.TokenForURL(idx.Url))
	if err != nil {
		return AddonVersion{}, err
	}
	return provider.Lookup(name, constraint, odooSeries)
}
