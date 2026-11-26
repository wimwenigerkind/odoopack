package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack/pkg/helper"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

const FileName = "odoopack.json"

const DefaultAddonsDir = "addons/odoopack"

func Load() (*Manifest, error) {
	exists, err := helper.FileExists(FileName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s not found, run 'odoopack init' first", FileName)
	}

	data, err := os.ReadFile(FileName)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	manifest.FormatVersion, err = helper.CheckFormatVersion(FileName, manifest.FormatVersion, FormatVersion)
	if err != nil {
		return nil, err
	}
	// The warning catches a misspelt key, which Save would otherwise keep
	// silently.
	for _, key := range manifest.UnknownKeys() {
		ui.Warn("%s: unknown key %q is kept but ignored by this odoopack", FileName, key)
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}

	return &manifest, nil
}

func Save(manifest Manifest) error {
	manifest.FormatVersion = FormatVersion
	// A backstop: commands check their input before they change anything.
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("refusing to write %s: %w", FileName, err)
	}
	data, err := json.Marshal(manifest, jsontext.WithIndent("  "), json.Deterministic(true))
	if err != nil {
		return err
	}
	return helper.WriteJSONFile(FileName, data)
}

func ProjectName(dir string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(filepath.Base(dir)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	project := strings.Trim(b.String(), "-_")
	if name.ValidPackage(project) != nil {
		return ""
	}
	return project
}

func Init(name, odoo string) (Manifest, error) {
	exists, err := helper.FileExists(FileName)
	if err != nil {
		return Manifest{}, err
	}
	if exists {
		return Manifest{}, fmt.Errorf("%s already exists", FileName)
	}

	addonsDir := viper.GetString("addons_dir")
	if addonsDir == "" {
		addonsDir = DefaultAddonsDir
	}
	manifest := NewManifest(name, addonsDir, odoo)
	if defaultURL := viper.GetString("default_index_url"); defaultURL != "" {
		manifest.Indexes = append(manifest.Indexes, Index{Name: DefaultIndexName, Url: defaultURL, Type: "registry"})
	}

	if err := Save(*manifest); err != nil {
		return Manifest{}, err
	}

	return *manifest, nil
}

func (m *Manifest) AddRequirement(name, version string) {
	if m.Require == nil {
		m.Require = make(Requirements)
	}

	m.Require[name] = version
}

func (m *Manifest) RemoveRequirement(name string) {
	delete(m.Require, name)
}
