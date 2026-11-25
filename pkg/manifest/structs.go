package manifest

import "encoding/json"

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
	OdooSeries    string       `json:"odoo_series"`
	Require       Requirements `json:"require"`
	Indexes       Indexes      `json:"indexes"`
	AddonsDir     string       `json:"addons_dir"`
}

// UnmarshalJSON also reads odoo and addons_path, which releases before
// odoo_series and addons_dir wrote. Save writes the new keys only, so the old
// ones go away with the next write.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	type plain Manifest
	var v struct {
		plain
		Odoo       string `json:"odoo"`
		AddonsPath string `json:"addons_path"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*m = Manifest(v.plain)
	if m.OdooSeries == "" {
		m.OdooSeries = v.Odoo
	}
	if m.AddonsDir == "" {
		m.AddonsDir = v.AddonsPath
	}
	return nil
}

func NewManifest(name, addonsDir, odooSeries string) *Manifest {
	return &Manifest{
		FormatVersion: FormatVersion,
		Name:          name,
		OdooSeries:    odooSeries,
		Require:       make(Requirements),
		Indexes:       Indexes{},
		AddonsDir:     addonsDir,
	}
}
