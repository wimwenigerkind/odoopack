package lockfile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/helper"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

func LoadOrNew() LockFile {
	lf, err := Load()
	if err != nil {
		return LockFile{
			Packages: make(map[string]LockedPackage),
		}
	}
	if lf.Packages == nil {
		lf.Packages = make(map[string]LockedPackage)
	}
	return lf
}

func Load() (LockFile, error) {
	exists, err := helper.FileExists(viper.GetString("lock"))
	if err != nil {
		return LockFile{}, err
	}
	if !exists {
		return LockFile{}, fmt.Errorf("odoopack.lock not found")
	}

	data, err := os.ReadFile(viper.GetString("lock"))
	if err != nil {
		return LockFile{}, err
	}

	var lockFile LockFile
	if err := json.Unmarshal(data, &lockFile); err != nil {
		return LockFile{}, err
	}

	return lockFile, nil
}

func Save(lockFile LockFile) error {
	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(viper.GetString("lock"), data, 0644)
}

func ComputeHash(require map[string]string, indexes manifest.Indexes, packages map[string]LockedPackage) (string, error) {
	payload := struct {
		Require  map[string]string        `json:"require"`
		Indexes  manifest.Indexes         `json:"indexes"`
		Packages map[string]LockedPackage `json:"packages"`
	}{Require: require, Indexes: indexes, Packages: packages}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash), nil
}

func IsStale(require map[string]string, indexes manifest.Indexes, packages map[string]LockedPackage, hash string) (bool, error) {
	computedHash, err := ComputeHash(require, indexes, packages)
	if err != nil {
		return false, err
	}
	return computedHash != hash, nil
}

type resolveItem struct {
	name       string
	constraint string
	direct     bool
}

func RecomputeHash(require map[string]string, indexes manifest.Indexes, odooSeries string) (LockFile, error) {
	packages := make(map[string]LockedPackage)

	queue := make([]resolveItem, 0, len(require))
	for name, version := range require {
		queue = append(queue, resolveItem{name: name, constraint: version, direct: true})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if _, seen := packages[item.name]; seen {
			continue
		}

		lookup, err := index.Lookup(indexes, item.name, item.constraint, odooSeries)
		if err != nil {
			return LockFile{}, err
		}

		var deps, external []string
		for _, dep := range lookup.Depends {
			switch dep.Access {
			case "forbidden":
				return LockFile{}, fmt.Errorf("%s requires %q (module %s) which you cannot access on the registry", lookup.Name, dep.Package, dep.Module)
			case "external":
				external = append(external, dep.Module)
			case "ok":
				if dep.Package == "" {
					continue
				}
				deps = append(deps, dep.Package)
				if _, seen := packages[dep.Package]; seen {
					continue
				}
				if odooSeries == "" {
					return LockFile{}, fmt.Errorf("cannot resolve transitive dependency %q without an Odoo series; set \"odoo\" in the manifest", dep.Package)
				}
				queue = append(queue, resolveItem{name: dep.Package, constraint: odooSeries, direct: false})
			}
		}

		packages[lookup.Name] = LockedPackage{
			Version: lookup.Version,
			Dist: Dist{
				Type:      lookup.Type,
				URL:       lookup.Repository,
				Reference: lookup.Reference,
				Shasum:    lookup.Shasum,
			},
			Direct:   item.direct,
			Depends:  deps,
			External: external,
		}
	}

	hash, err := ComputeHash(require, indexes, packages)
	if err != nil {
		return LockFile{}, err
	}

	return LockFile{
		ContentHash: hash,
		Packages:    packages,
	}, nil
}
