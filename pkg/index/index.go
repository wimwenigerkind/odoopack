package index

import (
	"fmt"
	"sort"

	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

type AddonVersion struct {
	Name       string
	Version    string
	Type       string
	Repository string
}

type Provider interface {
	Lookup(name, version string) (AddonVersion, error)
}

func NewProvider(repoType, url string) (Provider, error) {
	switch repoType {
	case "registry":
		return &RegistryProvider{BaseURL: url}, nil
	default:
		return nil, fmt.Errorf("unknown repository type %q", repoType)
	}
}

func Lookup(indexes manifest.Indexes, name, version string) (AddonVersion, error) {
	keys := make([]string, 0, len(indexes))
	for k := range indexes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		idx := indexes[k]
		provider, err := NewProvider(idx.Type, idx.Url)
		if err != nil {
			continue
		}
		result, err := provider.Lookup(name, version)
		if err == nil {
			return result, nil
		}
	}
	return AddonVersion{}, fmt.Errorf("addon %q@%s not found in any repository", name, version)
}
