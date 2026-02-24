package lockfile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/helper"
	"github.com/wimwenigerkind/odoopack/pkg/index"
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

func ComputeHash(require map[string]string) (string, error) {
	data, err := json.Marshal(require)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash), nil
}

func IsStale(require map[string]string, hash string) (bool, error) {
	computedHash, err := ComputeHash(require)
	if err != nil {
		return false, err
	}
	if computedHash != hash {
		return true, nil
	}
	return false, nil
}

func RecomputeHash(require map[string]string, provider index.Provider) (LockFile, error) {
	packages := make(map[string]LockedPackage)

	for name, version := range require {
		lookup, err := provider.Lookup(name, version)
		if err != nil {
			return LockFile{}, err
		}

		packages[lookup.Name] = LockedPackage{
			Version:    lookup.Version,
			Type:       lookup.Type,
			Repository: lookup.Repository,
		}
	}

	lockFile := LockFile{
		Packages: packages,
	}

	hash, err := ComputeHash(require)
	if err != nil {
		return LockFile{}, err
	}
	lockFile.ContentHash = hash

	return lockFile, nil
}
