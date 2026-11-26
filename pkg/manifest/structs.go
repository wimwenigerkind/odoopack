package manifest

import (
	"encoding/json/jsontext"
	"sort"
)

type Requirements map[string]string

type Index struct {
	Name    string                    `json:"name"`
	Url     string                    `json:"url"`
	Type    string                    `json:"type"`
	Unknown map[string]jsontext.Value `json:",embed"`
}

type Indexes []Index

const DefaultIndexName = "default"

func (ix Indexes) Find(name string) int {
	for i, idx := range ix {
		if idx.Name == name {
			return i
		}
	}
	return -1
}

// Add puts a new index first, like Composer, since it is usually a private
// registry that has to win over the public one. An existing one keeps its
// position and the keys a newer odoopack added, such as a filter.
func (ix Indexes) Add(idx Index, last bool) Indexes {
	switch i := ix.Find(idx.Name); {
	case i >= 0:
		idx.Unknown = ix[i].Unknown
		ix[i] = idx
		return ix
	case last:
		return append(ix, idx)
	default:
		return append(Indexes{idx}, ix...)
	}
}

func (ix Indexes) Remove(name string) (Indexes, bool) {
	i := ix.Find(name)
	if i < 0 {
		return ix, false
	}
	return append(ix[:i:i], ix[i+1:]...), true
}

// FormatVersion is raised only when an older odoopack would misread the file.
const FormatVersion = 1

type Manifest struct {
	Schema        string                    `json:"$schema,omitzero"`
	FormatVersion int                       `json:"format_version"`
	Name          string                    `json:"name,omitzero"`
	OdooSeries    string                    `json:"odoo_series"`
	Require       Requirements              `json:"require"`
	Indexes       Indexes                   `json:"indexes"`
	AddonsDir     string                    `json:"addons_dir"`
	Extra         jsontext.Value            `json:"extra,omitzero"`
	Unknown       map[string]jsontext.Value `json:",embed"`
}

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
