package index

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wimwenigerkind/odoopack/pkg/auth"
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

func NewProvider(repoType, url, token string) (Provider, error) {
	switch repoType {
	case "registry":
		return &RegistryProvider{BaseURL: url, Token: token}, nil
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

	var attempts []string
	for _, k := range keys {
		idx := indexes[k]
		provider, err := NewProvider(idx.Type, idx.Url, auth.TokenForURL(idx.Url))
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("%s[%s]: %v", k, idx.Url, err))
			continue
		}
		result, err := provider.Lookup(name, version)
		if err == nil {
			return result, nil
		}
		attempts = append(attempts, fmt.Sprintf("%s[%s]: %v", k, idx.Url, err))
	}
	if len(attempts) == 0 {
		return AddonVersion{}, fmt.Errorf("addon %q@%s: no indexes configured", name, version)
	}
	return AddonVersion{}, fmt.Errorf("addon %q@%s not found:\n  %s", name, version, strings.Join(attempts, "\n  "))
}
