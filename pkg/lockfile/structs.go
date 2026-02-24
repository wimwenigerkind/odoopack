package lockfile

type LockedPackage struct {
	Version    string `json:"version"`
	Type       string `json:"type"`
	Repository string `json:"repository"`
}

type LockFile struct {
	ContentHash string                   `json:"content_hash"`
	Packages    map[string]LockedPackage `json:"packages"`
}
