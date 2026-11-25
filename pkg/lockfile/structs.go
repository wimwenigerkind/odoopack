package lockfile

type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Digest    string `json:"digest,omitempty"`
}

type LockedPackage struct {
	Version         string   `json:"version"`
	Dist            Dist     `json:"dist"`
	Direct          bool     `json:"direct"`
	Depends         []string `json:"depends,omitempty"`
	ExternalModules []string `json:"external_modules,omitempty"`
	// IndexURL is the normalized URL of the index the package was resolved
	// from, so that later resolutions ask that index only, whatever it is
	// named in the manifest.
	IndexURL string `json:"index_url,omitempty"`
}

// FormatVersion is the lockfile format this odoopack writes. Raise it only
// when an older odoopack would misread the new one; adding an optional field
// does not need it.
const FormatVersion = 1

type LockFile struct {
	FormatVersion int                      `json:"format_version"`
	ContentHash   string                   `json:"content_hash"`
	Packages      map[string]LockedPackage `json:"packages"`
}
