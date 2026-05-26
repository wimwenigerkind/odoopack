package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type RegistryProvider struct {
	BaseURL string
}

type registryAddon struct {
	Name     string            `json:"name"`
	Versions []registryVersion `json:"versions"`
}

type registryVersion struct {
	Version string `json:"version"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Shasum  string `json:"shasum,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

func (p *RegistryProvider) Lookup(name, version string) (AddonVersion, error) {
	endpoint, err := url.Parse(strings.TrimRight(p.BaseURL, "/") + "/registry/v1/addons/" + name)
	if err != nil {
		return AddonVersion{}, fmt.Errorf("invalid registry url: %w", err)
	}

	response, err := http.Get(endpoint.String())
	if err != nil {
		return AddonVersion{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return AddonVersion{}, fmt.Errorf("addon %q not found at %s", name, p.BaseURL)
	}
	if response.StatusCode != http.StatusOK {
		return AddonVersion{}, fmt.Errorf("bad status %s", response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return AddonVersion{}, err
	}

	var addon registryAddon
	if err := json.Unmarshal(body, &addon); err != nil {
		return AddonVersion{}, err
	}

	for _, v := range addon.Versions {
		if v.Version == version {
			return AddonVersion{
				Name:       addon.Name,
				Version:    v.Version,
				Type:       v.Type,
				Repository: v.URL,
			}, nil
		}
	}
	return AddonVersion{}, fmt.Errorf("version %q not found for addon %q", version, name)
}
