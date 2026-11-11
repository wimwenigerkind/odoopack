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
}

// Format is the lockfile format this odoopack writes. Raise it only when an
// older odoopack would misread the new one; adding an optional field does not
// need it.
const Format = 1

type LockFile struct {
	// Format stays out of content_hash, so that a lockfile written before the
	// field existed is not stale once it gains it.
	Format      int                      `json:"format"`
	ContentHash string                   `json:"content_hash"`
	Packages    map[string]LockedPackage `json:"packages"`
}
