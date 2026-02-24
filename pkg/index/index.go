package index

import (
	"fmt"

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
	case "odoopack":
		return &OdoopackProvider{Endpoint: url}, nil
	default:
		return nil, fmt.Errorf("unknown repository type %q", repoType)
	}
}

func Lookup(indexes manifest.Indexes, name, version string) (AddonVersion, error) {
	for _, idx := range indexes {
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
