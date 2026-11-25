package manifest

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/helper"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

// FileName is the manifest of the project in the working directory. It is
// part of the format, like Composer's composer.json, so it is not
// configurable.
const FileName = "odoopack.json"

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
	// Save keeps these keys, so a key a newer odoopack added survives a
	// write by this one. The warning catches a misspelt key, which would
	// otherwise be ignored silently.
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
	// A command validates its input before it changes anything; this keeps a
	// mistake in one from writing a manifest no odoopack would load again.
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("refusing to write %s: %w", FileName, err)
	}
	data, err := json.Marshal(manifest, jsontext.WithIndent("  "), json.Deterministic(true))
	if err != nil {
		return err
	}
	return helper.WriteJSONFile(FileName, data)
}

func Init(name, odoo string) (Manifest, error) {
	exists, err := helper.FileExists(FileName)
	if err != nil {
		return Manifest{}, err
	}
	if exists {
		return Manifest{}, fmt.Errorf("%s already exists", FileName)
	}

	manifest := NewManifest(name, viper.GetString("addons_dir"), odoo)
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
