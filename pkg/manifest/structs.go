package manifest

type Requirements map[string]string

type Index struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}

type Indexes map[string]Index

// FormatVersion is the manifest format this odoopack writes. Raise it only
// when an older odoopack would misread the new one; adding an optional field
// does not need it.
const FormatVersion = 1

type Manifest struct {
	// FormatVersion stays out of content_hash, so that a lockfile does not
	// become stale when its manifest gains the field.
	FormatVersion int          `json:"format_version"`
	Name          string       `json:"name"`
	Odoo          string       `json:"odoo"`
	Require       Requirements `json:"require"`
	Indexes       Indexes      `json:"indexes"`
	AddonsPath    string       `json:"addons_path"`
}

func NewManifest(name, addonsPath, odoo string) *Manifest {
	return &Manifest{
		FormatVersion: FormatVersion,
		Name:          name,
		Odoo:          odoo,
		Require:       make(Requirements),
		Indexes:       Indexes{},
		AddonsPath:    addonsPath,
	}
}
