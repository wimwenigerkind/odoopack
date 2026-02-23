package manifest

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wimwenigerkind/odoopack/pkg/helper"
)

var manifestFilePath string = "odoopack.json"

func Load() (Manifest, error) {
	exists, err := helper.FileExists(manifestFilePath)
	if err != nil {
		return Manifest{}, err
	}
	if !exists {
		return Manifest{}, fmt.Errorf("odoopack.json not found, run 'odoopack init' first")
	}

	data, err := os.ReadFile(manifestFilePath)
	if err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

func Save(manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestFilePath, data, 0644)
}

func Init(name string) (Manifest, error) {
	exists, err := helper.FileExists(manifestFilePath)
	if err != nil {
		return Manifest{}, err
	}
	if exists {
		return Manifest{}, fmt.Errorf("odoopack.json already exists")
	}

	manifest := NewManifest(name)

	if err := Save(*manifest); err != nil {
		return Manifest{}, err
	}

	return *manifest, nil
}

func AddRequirement(name, version string) error {
	manifest, err := Load()
	if err != nil {
		return err
	}

	if manifest.Require == nil {
		manifest.Require = make(Requirements)
	}

	manifest.Require[name] = version

	return Save(manifest)
}
