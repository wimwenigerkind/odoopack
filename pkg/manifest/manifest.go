package manifest

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/helper"
)

func Load() (*Manifest, error) {
	exists, err := helper.FileExists(viper.GetString("manifest"))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("odoopack.json not found, run 'odoopack init' first")
	}

	data, err := os.ReadFile(viper.GetString("manifest"))
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func Save(manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(viper.GetString("manifest"), data, 0644)
}

func Init(name string) (Manifest, error) {
	exists, err := helper.FileExists(viper.GetString("manifest"))
	if err != nil {
		return Manifest{}, err
	}
	if exists {
		return Manifest{}, fmt.Errorf("odoopack.json already exists")
	}

	manifest := NewManifest(name, viper.GetString("index_url"), viper.GetString("addons_path"))

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
