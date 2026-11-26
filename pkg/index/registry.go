package index

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"github.com/wimwenigerkind/odoopack/pkg/httpclient"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

// maxMetadataBytes caps the metadata of one addon. It holds every version
// with its dependencies, which stays far below this even for an addon with
// hundreds of releases.
const maxMetadataBytes = 8 << 20

// httpClient is a variable so that tests can give it a short timeout.
var httpClient = httpclient.Metadata

type RegistryProvider struct {
	BaseURL string
	Token   string
}

func (p *RegistryProvider) Lookup(addonName, constraint, odooSeries string) (AddonVersion, error) {
	if err := name.ValidPackage(addonName); err != nil {
		return AddonVersion{}, err
	}

	// The server routes a vendor and a name as two path segments, so each
	// part is escaped on its own and the / between them stays.
	parts := strings.Split(addonName, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	endpoint, err := url.Parse(strings.TrimRight(p.BaseURL, "/") + "/registry/v1/addons/" + strings.Join(parts, "/"))
	if err != nil {
		return AddonVersion{}, fmt.Errorf("invalid registry url: %w", err)
	}

	req, err := httpclient.NewRequest(context.Background(), endpoint.String(), p.Token)
	if err != nil {
		return AddonVersion{}, err
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return AddonVersion{}, err
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return AddonVersion{}, fmt.Errorf("authentication required at %s, set ODOOPACK_AUTH%s", p.BaseURL, problemDetail(response))
	case http.StatusForbidden:
		return AddonVersion{}, fmt.Errorf("access denied at %s%s", p.BaseURL, problemDetail(response))
	case http.StatusNotFound:
		if p.Token == "" {
			return AddonVersion{}, fmt.Errorf("addon %q %w at %s (set ODOOPACK_AUTH if the registry is private)", addonName, ErrNotFound, p.BaseURL)
		}
		return AddonVersion{}, fmt.Errorf("addon %q %w at %s (or your token has no access)", addonName, ErrNotFound, p.BaseURL)
	default:
		return AddonVersion{}, fmt.Errorf("registry %s returned %s%s", p.BaseURL, response.Status, problemDetail(response))
	}

	body, err := httpclient.ReadAll(response.Body, maxMetadataBytes)
	if err != nil {
		return AddonVersion{}, fmt.Errorf("registry %s: %w", p.BaseURL, err)
	}

	var addon registryapi.Addon
	if err := json.Unmarshal(body, &addon); err != nil {
		return AddonVersion{}, err
	}
	if err := name.ValidPackage(addon.Name); err != nil {
		return AddonVersion{}, fmt.Errorf("registry %s answered for %q with an invalid name: %w", p.BaseURL, addonName, err)
	}
	if addon.Name != addonName {
		return AddonVersion{}, fmt.Errorf("registry %s answered for %q with addon %q", p.BaseURL, addonName, addon.Name)
	}

	versions, err := addon.ValidVersions()
	if err != nil {
		skipped := []error{err}
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			skipped = joined.Unwrap()
		}
		for _, e := range skipped {
			ui.Warn("%s: skipping %v", addonName, e)
		}
	}

	match, err := resolveVersion(constraint, odooSeries, versions)
	if err != nil {
		return AddonVersion{}, fmt.Errorf("%s: %w", addonName, err)
	}
	deps := make([]Dep, 0, len(match.Depends))
	for _, d := range match.Depends {
		deps = append(deps, Dep(d))
	}
	return AddonVersion{
		Name:       addon.Name,
		Version:    match.Version,
		Type:       match.Type,
		Repository: match.URL,
		Reference:  match.Reference,
		Digest:     match.Digest,
		Depends:    deps,
	}, nil
}

// problemDetail returns the detail of an application/problem+json body as a
// suffix for an error message, or nothing for any other body.
func problemDetail(response *http.Response) string {
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/problem+json" {
		return ""
	}
	var problem struct {
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&problem); err != nil || problem.Detail == "" {
		return ""
	}
	return ": " + problem.Detail
}
