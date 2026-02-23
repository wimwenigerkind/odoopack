package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type StaticProvider struct {
	Endpoint string
}

type staticResponse struct {
	Addons map[string]staticAddon `json:"addons"`
}

type staticAddon struct {
	Versions map[string]staticVersion `json:"versions"`
}

type staticVersion struct {
	Type       Type   `json:"type"`
	Repository string `json:"repository"`
}

func (p *StaticProvider) Lookup(name, version string) (AddonVersion, error) {
	response, err := http.Get(p.Endpoint)
	if err != nil {
		return AddonVersion{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return AddonVersion{}, err
	}

	var index staticResponse
	err = json.Unmarshal(body, &index)
	if err != nil {
		return AddonVersion{}, err
	}

	addon, ok := index.Addons[name]
	if !ok {
		return AddonVersion{}, fmt.Errorf("addon %q not found in index", name)
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
