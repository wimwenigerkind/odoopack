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
	Token   string
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

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return AddonVersion{}, err
	}
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return AddonVersion{}, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return AddonVersion{}, fmt.Errorf("authentication required at %s — set ODOOPACK_AUTH", p.BaseURL)
	case http.StatusForbidden:
		return AddonVersion{}, fmt.Errorf("access denied at %s", p.BaseURL)
	case http.StatusNotFound:
		if p.Token == "" {
			return AddonVersion{}, fmt.Errorf("addon %q not found at %s (set ODOOPACK_AUTH if the registry is private)", name, p.BaseURL)
		}
		return AddonVersion{}, fmt.Errorf("addon %q not found at %s (or your token has no access)", name, p.BaseURL)
	default:
		return AddonVersion{}, fmt.Errorf("registry %s returned %s", p.BaseURL, response.Status)
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
