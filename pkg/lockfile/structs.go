package lockfile

type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Shasum    string `json:"shasum,omitempty"`
}

type LockedPackage struct {
	Version string `json:"version"`
	Dist    Dist   `json:"dist"`
}

type LockFile struct {
	ContentHash string                   `json:"content_hash"`
	Packages    map[string]LockedPackage `json:"packages"`
}
