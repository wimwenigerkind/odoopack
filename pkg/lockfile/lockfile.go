package lockfile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/digest"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack/pkg/helper"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

// LoadOrNew returns an empty lockfile when none exists. Any other problem is
// an error, so that a lockfile with an invalid name is not silently replaced.
func LoadOrNew() (LockFile, error) {
	exists, err := helper.FileExists(viper.GetString("lock"))
	if err != nil {
		return LockFile{}, err
	}
	if !exists {
		return LockFile{Format: Format, Packages: make(map[string]LockedPackage)}, nil
	}
	lf, err := Load()
	if err != nil {
		return LockFile{}, err
	}
	if lf.Packages == nil {
		lf.Packages = make(map[string]LockedPackage)
	}
	return lf, nil
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
	switch {
	// Every lockfile written before the field existed is format 1.
	case lockFile.Format == 0:
		lockFile.Format = 1
	case lockFile.Format < 0:
		return LockFile{}, fmt.Errorf("%s: invalid format %d", viper.GetString("lock"), lockFile.Format)
	case lockFile.Format > Format:
		return LockFile{}, fmt.Errorf("%s has format %d, but this odoopack only reads up to format %d; upgrade odoopack", viper.GetString("lock"), lockFile.Format, Format)
	}
	for pkg := range lockFile.Packages {
		if err := name.ValidPackage(pkg); err != nil {
			return LockFile{}, fmt.Errorf("%s: %w", viper.GetString("lock"), err)
		}
	}

	return lockFile, nil
}

func Save(lockFile LockFile) error {
	lockFile.Format = Format
	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(viper.GetString("lock"), data, 0644)
}

// ComputeHash covers everything the packages were resolved from, so that a
// change to any of it makes the lockfile stale.
func ComputeHash(require map[string]string, indexes manifest.Indexes, odoo string, packages map[string]LockedPackage) (string, error) {
	return hashJSON(struct {
		Require  map[string]string        `json:"require"`
		Indexes  manifest.Indexes         `json:"indexes"`
		Odoo     string                   `json:"odoo"`
		Packages map[string]LockedPackage `json:"packages"`
	}{Require: require, Indexes: indexes, Odoo: odoo, Packages: packages})
}

// legacyHash is content_hash as odoopack computed it before the series was
// part of it.
func legacyHash(require map[string]string, indexes manifest.Indexes, packages map[string]LockedPackage) (string, error) {
	return hashJSON(struct {
		Require  map[string]string        `json:"require"`
		Indexes  manifest.Indexes         `json:"indexes"`
		Packages map[string]LockedPackage `json:"packages"`
	}{Require: require, Indexes: indexes, Packages: packages})
}

func hashJSON(payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash), nil
}

func IsStale(require map[string]string, indexes manifest.Indexes, odoo string, packages map[string]LockedPackage, hash string) (bool, error) {
	computedHash, err := ComputeHash(require, indexes, odoo, packages)
	if err != nil {
		return false, err
	}
	if computedHash == hash {
		return false, nil
	}

	// A lockfile hashed before the series was part of content_hash is still
	// fresh, so that upgrading odoopack does not make install --frozen-lockfile
	// fail in every project. Its series cannot be checked until the lockfile is
	// written again, which then adds it to the hash.
	legacy, err := legacyHash(require, indexes, packages)
	if err != nil {
		return false, err
	}
	return legacy != hash, nil
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

		locked, err := lockPackage(lookup, item.direct, deps, external)
		if err != nil {
			return LockFile{}, err
		}
		packages[lookup.Name] = locked
	}

	hash, err := ComputeHash(require, indexes, odooSeries, packages)
	if err != nil {
		return LockFile{}, err
	}

	return LockFile{
		ContentHash: hash,
		Packages:    packages,
	}, nil
}

// lockPackage refuses a version whose content cannot be verified at install,
// whatever index it came from.
func lockPackage(lookup index.AddonVersion, direct bool, deps, external []string) (LockedPackage, error) {
	pkg := LockedPackage{
		Version: lookup.Version,
		Dist: Dist{
			Type:      lookup.Type,
			URL:       lookup.Repository,
			Reference: lookup.Reference,
			Shasum:    lookup.Shasum,
		},
		Direct:   direct,
		Depends:  deps,
		External: external,
	}
	if err := pkg.Dist.CheckIntegrity(); err != nil {
		return LockedPackage{}, fmt.Errorf("%s %s: %w", lookup.Name, lookup.Version, err)
	}
	return pkg, nil
}

// CheckIntegrity reports whether d pins its content: a zip by its sha256
// shasum, a git package by a full commit id.
func (d Dist) CheckIntegrity() error {
	switch d.Type {
	case "zip":
		if d.Shasum == "" {
			return fmt.Errorf("zip has no shasum to verify it against")
		}
		_, err := digest.Parse(d.Shasum)
		return err
	case "git":
		if d.Reference == "" {
			return fmt.Errorf("git has no reference to check out")
		}
		if !isCommit(d.Reference) {
			return fmt.Errorf("git reference %q must be a full commit id, 40 or 64 lowercase hex characters", d.Reference)
		}
		return nil
	}
	return fmt.Errorf("type %q must be \"zip\" or \"git\"", d.Type)
}

// isCommit matches what the registry accepts as a reference: 40 hex
// characters for sha1 repositories, 64 for sha256 ones.
func isCommit(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
