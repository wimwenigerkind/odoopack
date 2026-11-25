package lockfile

type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Shasum    string `json:"shasum,omitempty"`
}

type LockedPackage struct {
	Version  string   `json:"version"`
	Dist     Dist     `json:"dist"`
	Direct   bool     `json:"direct"`
	Depends  []string `json:"depends,omitempty"`
	External []string `json:"external,omitempty"`
	// Registry is the normalized URL of the index the package was resolved
	// from, so that later resolutions ask that index only, whatever it is
	// named in the manifest. Lockfiles written before the field existed lack
	// it.
	Registry string `json:"registry,omitempty"`
	// Index is the name of that index, which lockfiles held before Registry.
	// It is only read: resolving the package again writes Registry instead.
	Index string `json:"index,omitempty"`
}

// LockedTo is what the package is locked to: the URL of its index or, in an
// older lockfile, the name.
func (p LockedPackage) LockedTo() string {
	if p.Registry != "" {
		return p.Registry
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
