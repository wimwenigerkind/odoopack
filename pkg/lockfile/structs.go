package lockfile

import "encoding/json/jsontext"

type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Digest    string `json:"digest,omitempty"`
	// Unknown holds keys a newer odoopack wrote, such as a signature, which
	// this one cannot compute again. They stay as long as the package is
	// kept as it is; resolving it again drops them.
	Unknown map[string]jsontext.Value `json:",embed"`
}

// Same reports whether d and o download the same content. Keys this
// odoopack does not know play no part.
func (d Dist) Same(o Dist) bool {
	return d.Type == o.Type && d.URL == o.URL && d.Reference == o.Reference && d.Digest == o.Digest
}

type LockedPackage struct {
	Version string `json:"version"`
	// Module is the directory the package installs to, the technical name
	// of its Odoo module. Every command takes it from here instead of
	// deriving it from the package name, so the rule that derives it can
	// change, and a package can name a module of another name, without
	// moving an installed addon.
	Module          string   `json:"module"`
	Dist            Dist     `json:"dist"`
	Direct          bool     `json:"direct"`
	Depends         []string `json:"depends,omitempty"`
	ExternalModules []string `json:"external_modules,omitempty"`
	// IndexURL is the URL of the index the package was resolved from, as
	// the manifest writes it, so that later resolutions ask that index only,
	// whatever it is named.
	IndexURL string `json:"index_url,omitempty"`
	// Unknown holds keys a newer odoopack wrote for the package, kept like
	// those of Dist.
	Unknown map[string]jsontext.Value `json:",embed"`
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
