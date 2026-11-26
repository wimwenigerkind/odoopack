package lockfile

import "encoding/json/jsontext"

type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Digest    string `json:"digest,omitempty"`
	// Unknown keeps the keys of a newer odoopack while the package stays as
	// it is.
	Unknown map[string]jsontext.Value `json:",embed"`
}

// Same ignores the keys this odoopack does not know.
func (d Dist) Same(o Dist) bool {
	return d.Type == o.Type && d.URL == o.URL && d.Reference == o.Reference && d.Digest == o.Digest
}

type LockedPackage struct {
	Version string `json:"version"`
	// Module is never derived from the package name again, so that the
	// derivation can change without moving an installed addon.
	Module          string                    `json:"module"`
	Dist            Dist                      `json:"dist"`
	Direct          bool                      `json:"direct"`
	Depends         []string                  `json:"depends,omitempty"`
	ExternalModules []string                  `json:"external_modules,omitempty"`
	IndexURL        string                    `json:"index_url,omitempty"`
	Unknown         map[string]jsontext.Value `json:",embed"`
}

// FormatVersion is raised only when an older odoopack would misread the file.
const FormatVersion = 1

type LockFile struct {
	FormatVersion int                      `json:"format_version"`
	ContentHash   string                   `json:"content_hash"`
	Packages      map[string]LockedPackage `json:"packages"`
}
