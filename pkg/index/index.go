package index

import (
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

func Lookup(indexes manifest.Indexes, name, constraint, odooSeries string) (AddonVersion, error) {
	keys := make([]string, 0, len(indexes))
	for k := range indexes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var attempts []string
	tryIndex := func(label string, idx manifest.Index) (AddonVersion, bool) {
		provider, err := NewProvider(idx.Type, idx.Url, auth.TokenForURL(idx.Url))
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("%s[%s]: %v", label, idx.Url, err))
			return AddonVersion{}, false
		}
		result, err := provider.Lookup(name, constraint, odooSeries)
		if err == nil {
			return result, true
		}
		attempts = append(attempts, fmt.Sprintf("%s[%s]: %v", label, idx.Url, err))
		return AddonVersion{}, false
	}

	for _, k := range keys {
		if result, ok := tryIndex(k, indexes[k]); ok {
			return result, nil
		}
	}

	defaultURL := viper.GetString("default_index_url")
	if defaultURL != "" && shouldUseImplicitDefault(indexes, defaultURL) {
		if result, ok := tryIndex("default", manifest.Index{Url: defaultURL, Type: "registry"}); ok {
			return result, nil
		}
	}

	if len(attempts) == 0 {
		return AddonVersion{}, fmt.Errorf("addon %q@%s: no indexes configured", name, constraint)
	}
	return AddonVersion{}, fmt.Errorf("addon %q@%s not found:\n  %s", name, constraint, strings.Join(attempts, "\n  "))
}

func shouldUseImplicitDefault(indexes manifest.Indexes, defaultURL string) bool {
	if _, hasDefault := indexes["default"]; hasDefault {
		return false
	}
	for _, idx := range indexes {
		if idx.Url == defaultURL {
			return false
		}
	}
	return true
}
