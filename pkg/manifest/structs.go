package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	Unknown map[string]json.RawMessage `json:"-"`
}

// Indexes are searched in their order: the first index that has an addon
// provides it. Order is part of the format, not of the names, because it
// decides which registry a name resolves to when two have it.
type Indexes []Index

// DefaultIndexName is the name init gives the built-in default registry,
// and the name an index from releases that searched it implicitly gets.
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

// UnmarshalJSON reads the array and, from releases before it, the object of
// indexes by name, which those releases searched in alphabetical order.
func (ix *Indexes) UnmarshalJSON(data []byte) error {
	if isObject(data) {
		var byName map[string]Index
		if err := json.Unmarshal(data, &byName); err != nil {
			return err
		}
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		*ix = make(Indexes, 0, len(names))
		for _, name := range names {
			idx := byName[name]
			idx.Name = name
			*ix = append(*ix, idx)
		}
		return nil
	}

	var list []Index
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	seen := make(map[string]bool, len(list))
	for _, idx := range list {
		if idx.Name == "" {
			return fmt.Errorf("indexes: every index needs a name")
		}
		if seen[idx.Name] {
			return fmt.Errorf("indexes: the name %q is used twice", idx.Name)
		}
		seen[idx.Name] = true
	}
	*ix = list
	return nil
}

func (i *Index) UnmarshalJSON(data []byte) error {
	type plain Index
	if err := json.Unmarshal(data, (*plain)(i)); err != nil {
		return err
	}
	unknown, err := unknownKeys(data, "name", "url", "type")
	if err != nil {
		return err
	}
	i.Unknown = unknown
	return nil
}

func (i Index) MarshalJSON() ([]byte, error) {
	type plain Index
	return withUnknown(plain(i), i.Unknown)
}

// FormatVersion is the manifest format this odoopack writes. Raise it only
// when an older odoopack would misread the new one; adding an optional field
// does not need it.
const FormatVersion = 1

type Manifest struct {
	// Schema is the URL of a JSON Schema for editors. odoopack does not read
	// it.
	Schema string `json:"$schema,omitempty"`
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
	Extra json.RawMessage `json:"extra,omitempty"`
	// Unknown holds the top-level keys this odoopack does not know, so that
	// Save writes back what a newer odoopack added.
	Unknown map[string]json.RawMessage `json:"-"`
	// legacyIndexes is set when indexes were missing or an object, written
	// by a release that searched the default registry implicitly.
	legacyIndexes bool
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
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	indexes, ok := raw["indexes"]
	m.legacyIndexes = !ok || isObject(indexes)
	if len(m.Extra) > 0 && !isObject(m.Extra) {
		return fmt.Errorf("extra must be an object")
	}
	unknown, err := unknownKeys(data, "$schema", "format_version", "name", "odoo_series", "require", "indexes", "addons_dir", "extra", "odoo", "addons_path")
	if err != nil {
		return err
	}
	m.Unknown = unknown
	return nil
}

func (m Manifest) MarshalJSON() ([]byte, error) {
	type plain Manifest
	return withUnknown(plain(m), m.Unknown)
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

// unknownKeys returns the keys of the JSON object data that are not known,
// or nil when there are none.
func unknownKeys(data []byte, known ...string) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for _, key := range known {
		delete(all, key)
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

// withUnknown marshals known, which must marshal to a non-empty object, and
// appends the unknown keys sorted, so that the output does not depend on the
// order they were read in.
func withUnknown(known any, unknown map[string]json.RawMessage) ([]byte, error) {
	data, err := json.Marshal(known)
	if err != nil || len(unknown) == 0 {
		return data, err
	}
	keys := make([]string, 0, len(unknown))
	for key := range unknown {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buf := bytes.NewBuffer(data[:len(data)-1])
	for _, key := range keys {
		name, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.WriteByte(',')
		buf.Write(name)
		buf.WriteByte(':')
		buf.Write(unknown[key])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func isObject(data json.RawMessage) bool {
	var obj map[string]json.RawMessage
	return json.Unmarshal(data, &obj) == nil && obj != nil
}

// migrateIndexes makes the default registry, which releases before the
// array of indexes searched after the others unless an index was named
// default or had its URL, an index of its own, so that resolution stays the
// same.
func (m *Manifest) migrateIndexes(defaultURL string) {
	if !m.legacyIndexes || defaultURL == "" || m.Indexes.Find(DefaultIndexName) >= 0 {
		return
	}
	for _, idx := range m.Indexes {
		if idx.Url == defaultURL {
			return
		}
	}
	m.Indexes = append(m.Indexes, Index{Name: DefaultIndexName, Url: defaultURL, Type: "registry"})
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
