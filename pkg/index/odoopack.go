package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OdoopackProvider struct {
	Endpoint string
}

type odoopackResponse struct {
	Addons map[string]odoopackAddon `json:"addons"`
}

type odoopackAddon struct {
	Versions map[string]odoopackVersion `json:"versions"`
}

type odoopackVersion struct {
	Type       string `json:"type"`
	Repository string `json:"repository"`
}

func (p *OdoopackProvider) Lookup(name, version string) (AddonVersion, error) {
	response, err := http.Get(p.Endpoint)
	if err != nil {
		return AddonVersion{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return AddonVersion{}, err
	}

	var idx odoopackResponse
	err = json.Unmarshal(body, &idx)
	if err != nil {
		return AddonVersion{}, err
	}

	addon, ok := idx.Addons[name]
	if !ok {
		return AddonVersion{}, fmt.Errorf("addon %q not found in index %s", name, p.Endpoint)
	}

	ver, ok := addon.Versions[version]
	if !ok {
		return AddonVersion{}, fmt.Errorf("version %q not found for addon %q", version, name)
	}

	return AddonVersion{
		Name:       name,
		Version:    version,
		Type:       ver.Type,
		Repository: ver.Repository,
	}, nil
}
