package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type Requirements map[string]string

type Index struct {
	Url  string `json:"url"`
	Type string `json:"type"`
	// Unknown holds the keys of the index this odoopack does not know, so
	// that Save writes back what a newer odoopack added.
	Unknown map[string]json.RawMessage `json:"-"`
}

type Indexes map[string]Index

func (i *Index) UnmarshalJSON(data []byte) error {
	type plain Index
	if err := json.Unmarshal(data, (*plain)(i)); err != nil {
		return err
	}
	unknown, err := unknownKeys(data, "url", "type")
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
	for name, idx := range m.Indexes {
		for key := range idx.Unknown {
			keys = append(keys, "indexes."+name+"."+key)
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
