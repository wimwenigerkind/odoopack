package manifest

import (
	"encoding/json/jsontext"
	"sort"
)

type Requirements map[string]string

type Index struct {
	// Name labels the index in messages. Resolution does not depend on it:
	// a package is locked to the URL of its index.
	Name string `json:"name"`
	Url  string `json:"url"`
	Type string `json:"type"`
	// Unknown holds the keys of the index this odoopack does not know, so
	// that Save writes back what a newer odoopack added.
	Unknown map[string]jsontext.Value `json:",embed"`
}

// Indexes are searched in their order: the first index that has an addon
// provides it. Order is part of the format, not of the names, because it
// decides which registry a name resolves to when two have it.
type Indexes []Index

// DefaultIndexName is the name init gives the built-in default registry.
const DefaultIndexName = "default"

// Find returns the position of the index named name, or -1.
func (ix Indexes) Find(name string) int {
	for i, idx := range ix {
		if idx.Name == name {
			return i
		}
	}
	return -1
}

// Add replaces the index with the name of idx where it is. A new index goes
// first like Composer adds a repository, since it is usually a private
// registry that has to win over the public one for the same name, or last
// when last is set.
func (ix Indexes) Add(idx Index, last bool) Indexes {
	switch i := ix.Find(idx.Name); {
	case i >= 0:
		ix[i] = idx
		return ix
	case last:
		return append(ix, idx)
	default:
		return append(Indexes{idx}, ix...)
	}
}

// Remove drops the index named name and reports whether there was one.
func (ix Indexes) Remove(name string) (Indexes, bool) {
	i := ix.Find(name)
	if i < 0 {
		return ix, false
	}
	return append(ix[:i:i], ix[i+1:]...), true
}

// FormatVersion is the manifest format this odoopack writes. Raise it only
// when an older odoopack would misread the new one; adding an optional field
// does not need it.
const FormatVersion = 1

type Manifest struct {
	// Schema is the URL of a JSON Schema for editors. odoopack does not read
	// it.
	Schema string `json:"$schema,omitzero"`
	// FormatVersion stays out of content_hash, so that a lockfile does not
	// become stale when its manifest gains the field.
	FormatVersion int          `json:"format_version"`
	Name          string       `json:"name"`
	OdooSeries    string       `json:"odoo_series"`
	Require       Requirements `json:"require"`
	Indexes       Indexes      `json:"indexes"`
	AddonsDir     string       `json:"addons_dir"`
	// Extra is a free-form object for data of other tools, which odoopack
	// keeps as it is and never warns about.
	Extra jsontext.Value `json:"extra,omitzero"`
	// Unknown holds the top-level keys this odoopack does not know, so that
	// Save writes back what a newer odoopack added.
	Unknown map[string]jsontext.Value `json:",embed"`
}

// UnknownKeys lists the keys of m this odoopack does not know, the keys of
// an index as indexes.<name>.<key>, sorted.
func (m Manifest) UnknownKeys() []string {
	var keys []string
	for key := range m.Unknown {
		keys = append(keys, key)
	}
	for _, idx := range m.Indexes {
		for key := range idx.Unknown {
			keys = append(keys, "indexes."+idx.Name+"."+key)
		}
	}
	sort.Strings(keys)
	return keys
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
