package lockfile

import "encoding/json"

type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

// UnmarshalJSON also reads shasum, which releases before digest wrote.
func (d *Dist) UnmarshalJSON(data []byte) error {
	type plain Dist
	var v struct {
		plain
		Shasum string `json:"shasum"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*d = Dist(v.plain)
	if d.Digest == "" {
		d.Digest = v.Shasum
	}
	return nil
}

type LockedPackage struct {
	Version         string   `json:"version"`
	Dist            Dist     `json:"dist"`
	Direct          bool     `json:"direct"`
	Depends         []string `json:"depends,omitempty"`
	ExternalModules []string `json:"external_modules,omitempty"`
	// IndexURL is the normalized URL of the index the package was resolved
	// from, so that later resolutions ask that index only, whatever it is
	// named in the manifest. Lockfiles written before the field existed lack
	// it.
	IndexURL string `json:"index_url,omitempty"`
	// Index is the name of that index, which lockfiles held before IndexURL.
	// It is only read: resolving the package again writes IndexURL instead.
	Index string `json:"index,omitempty"`
}

// UnmarshalJSON also reads external and registry, which releases before
// external_modules and index_url wrote. Save writes the new keys only, so the
// old ones go away with the next write.
func (p *LockedPackage) UnmarshalJSON(data []byte) error {
	type plain LockedPackage
	var v struct {
		plain
		External []string `json:"external"`
		Registry string   `json:"registry"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = LockedPackage(v.plain)
	if p.ExternalModules == nil {
		p.ExternalModules = v.External
	}
	if p.IndexURL == "" {
		p.IndexURL = v.Registry
	}
	return nil
}

// LockedTo is what the package is locked to: the URL of its index or, in an
// older lockfile, the name.
func (p LockedPackage) LockedTo() string {
	if p.IndexURL != "" {
		return p.IndexURL
	}
	return p.Index
}

// FormatVersion is the lockfile format this odoopack writes. Raise it only
// when an older odoopack would misread the new one; adding an optional field
// does not need it.
const FormatVersion = 1

type LockFile struct {
	// FormatVersion stays out of content_hash, so that a lockfile written
	// before the field existed is not stale once it gains it.
	FormatVersion int                      `json:"format_version"`
	ContentHash   string                   `json:"content_hash"`
	Packages      map[string]LockedPackage `json:"packages"`
}
