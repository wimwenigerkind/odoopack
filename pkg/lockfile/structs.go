package lockfile

import "github.com/wimwenigerkind/odoopack/pkg/index"

type LockedPackage struct {
	Version    string     `json:"version"`
	Type       index.Type `json:"type"`
	Repository string     `json:"repository"`
}

type LockFile struct {
	ContentHash string                   `json:"content_hash"`
	Packages    map[string]LockedPackage `json:"packages"`
}
