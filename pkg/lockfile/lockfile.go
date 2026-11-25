package lockfile

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-spec/digest"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/version"
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
		return LockFile{FormatVersion: FormatVersion, Packages: make(map[string]LockedPackage)}, nil
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
	if lockFile.FormatVersion == 0 {
		// Releases before format_version wrote the same number as "format".
		// Save writes format_version only, so the old key goes away with the
		// next write.
		var legacy struct {
			Format int `json:"format"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return LockFile{}, err
		}
		lockFile.FormatVersion = legacy.Format
	}
	lockFile.FormatVersion, err = helper.CheckFormatVersion(viper.GetString("lock"), lockFile.FormatVersion, FormatVersion)
	if err != nil {
		return LockFile{}, err
	}
	for pkg := range lockFile.Packages {
		if err := name.ValidPackage(pkg); err != nil {
			return LockFile{}, fmt.Errorf("%s: %w", viper.GetString("lock"), err)
		}
	}
	if err := CheckModuleCollisions(lockFile.Packages); err != nil {
		return LockFile{}, fmt.Errorf("%s: %w", viper.GetString("lock"), err)
	}

	return lockFile, nil
}

// CheckModuleCollisions refuses packages that install into the same addon
// directory, since one would silently overwrite the other: name.ModuleName
// maps both "/" and "-" to "_".
func CheckModuleCollisions(packages map[string]LockedPackage) error {
	names := make([]string, 0, len(packages))
	for pkg := range packages {
		names = append(names, pkg)
	}
	sort.Strings(names)

	owners := make(map[string]string, len(names))
	for _, pkg := range names {
		dir := name.ModuleName(pkg)
		if other, taken := owners[dir]; taken {
			return fmt.Errorf("packages %s and %s both install to the addon directory %s", other, pkg, dir)
		}
		owners[dir] = pkg
	}
	return nil
}

func Save(lockFile LockFile) error {
	lockFile.FormatVersion = FormatVersion
	data, err := json.MarshalIndent(lockFile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(viper.GetString("lock"), data, 0644)
}

// ComputeHash covers everything the packages were resolved from, so that a
// change to any of it makes the lockfile stale. The indexes take part by URL
// and type only: packages are locked to the URL, so renaming an index leaves
// the lockfile fresh.
func ComputeHash(require map[string]string, indexes manifest.Indexes, odoo string, packages map[string]LockedPackage) (string, error) {
	return hashJSON(struct {
		Require  map[string]string        `json:"require"`
		Indexes  []manifest.Index         `json:"indexes"`
		Odoo     string                   `json:"odoo"`
		Packages map[string]hashedPackage `json:"packages"`
	}{Require: require, Indexes: hashedIndexes(indexes), Odoo: odoo, Packages: hashedPackages(packages)})
}

// hashedPackage is a locked package as content_hash covers it. It keeps the
// keys the lockfile had before they were renamed, so that renaming a key in
// the lockfile does not make every lockfile stale.
type hashedPackage struct {
	Version  string     `json:"version"`
	Dist     hashedDist `json:"dist"`
	Direct   bool       `json:"direct"`
	Depends  []string   `json:"depends,omitempty"`
	External []string   `json:"external,omitempty"`
	Registry string     `json:"registry,omitempty"`
	Index    string     `json:"index,omitempty"`
}

type hashedDist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Shasum    string `json:"shasum,omitempty"`
}

func hashedPackages(packages map[string]LockedPackage) map[string]hashedPackage {
	hashed := make(map[string]hashedPackage, len(packages))
	for name, p := range packages {
		hashed[name] = hashedPackage{
			Version:  p.Version,
			Dist:     hashedDist{Type: p.Dist.Type, URL: p.Dist.URL, Reference: p.Dist.Reference, Shasum: p.Dist.Digest},
			Direct:   p.Direct,
			Depends:  p.Depends,
			External: p.ExternalModules,
			Registry: p.IndexURL,
			Index:    p.Index,
		}
	}
	return hashed
}

func hashedIndexes(indexes manifest.Indexes) []manifest.Index {
	list := make([]manifest.Index, 0, len(indexes))
	for _, idx := range indexes {
		list = append(list, manifest.Index{Url: index.NormalizeURL(idx.Url), Type: idx.Type})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Url != list[j].Url {
			return list[i].Url < list[j].Url
		}
		return list[i].Type < list[j].Type
	})
	return list
}

// namedHash is content_hash as odoopack computed it while packages were
// locked to the name of their index.
func namedHash(require map[string]string, indexes manifest.Indexes, odoo string, packages map[string]LockedPackage) (string, error) {
	return hashJSON(struct {
		Require  map[string]string        `json:"require"`
		Indexes  manifest.Indexes         `json:"indexes"`
		Odoo     string                   `json:"odoo"`
		Packages map[string]hashedPackage `json:"packages"`
	}{Require: require, Indexes: indexes, Odoo: odoo, Packages: hashedPackages(packages)})
}

// legacyHash is content_hash as odoopack computed it before the series was
// part of it.
func legacyHash(require map[string]string, indexes manifest.Indexes, packages map[string]LockedPackage) (string, error) {
	return hashJSON(struct {
		Require  map[string]string        `json:"require"`
		Indexes  manifest.Indexes         `json:"indexes"`
		Packages map[string]hashedPackage `json:"packages"`
	}{Require: require, Indexes: indexes, Packages: hashedPackages(packages)})
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

	// Lockfiles hashed the way an older odoopack did are still fresh, so that
	// upgrading odoopack does not make install --frozen-lockfile fail in every
	// project. The next write hashes them the current way.
	named, err := namedHash(require, indexes, odoo, packages)
	if err != nil {
		return false, err
	}
	if named == hash {
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

// requirement is one reason to lock a package: the manifest requires it, or a
// locked package depends on it.
type requirement struct {
	constraint string
	// path leads from a requirement of the manifest to the package that
	// depends on this one. It is empty for the manifest itself.
	path []string
}

func (r requirement) String() string {
	if len(r.path) == 0 {
		return "the manifest"
	}
	return strings.Join(r.path, " -> ")
}

type resolveItem struct {
	name string
	req  requirement
	// pinned keeps the locked version of the package and of the dependencies
	// it reaches, as long as they still satisfy their requirement.
	pinned bool
}

type resolver struct {
	require    map[string]string
	indexes    manifest.Indexes
	odooSeries string
	locked     map[string]LockedPackage
	packages   map[string]LockedPackage
	// chosen holds the requirement each package was resolved by, to name it
	// when another requirement conflicts.
	chosen map[string]requirement
}

// RecomputeHash resolves require again. A package in locked is resolved from
// the index it is locked to.
func RecomputeHash(require map[string]string, indexes manifest.Indexes, odooSeries string, locked map[string]LockedPackage) (LockFile, error) {
	r := newResolver(require, indexes, odooSeries, locked)
	if err := r.visit(r.directs(false)); err != nil {
		return LockFile{}, err
	}
	return r.lockFile()
}

// Update resolves the requirement name and the packages it depends on again,
// and keeps every other locked package that still satisfies its requirements.
func Update(require map[string]string, indexes manifest.Indexes, odooSeries string, locked map[string]LockedPackage, name string) (LockFile, error) {
	constraint, ok := require[name]
	if !ok {
		return LockFile{}, fmt.Errorf("%s is not a required addon", name)
	}
	r := newResolver(require, indexes, odooSeries, locked)
	// The tree of name goes first, so that a dependency it shares with a
	// pinned package is resolved again rather than kept.
	if err := r.visit([]resolveItem{{name: name, req: requirement{constraint: constraint}}}); err != nil {
		return LockFile{}, err
	}
	if err := r.visit(r.directs(true)); err != nil {
		return LockFile{}, err
	}
	return r.lockFile()
}

// Relock locks require keeping every locked package that still satisfies its
// requirements, so that it only asks an index for what is missing. Packages
// that nothing requires any more are left out.
func Relock(require map[string]string, indexes manifest.Indexes, odooSeries string, locked map[string]LockedPackage) (LockFile, error) {
	r := newResolver(require, indexes, odooSeries, locked)
	if err := r.visit(r.directs(true)); err != nil {
		return LockFile{}, err
	}
	return r.lockFile()
}

func newResolver(require map[string]string, indexes manifest.Indexes, odooSeries string, locked map[string]LockedPackage) *resolver {
	return &resolver{
		require:    require,
		indexes:    indexes,
		odooSeries: odooSeries,
		locked:     locked,
		packages:   make(map[string]LockedPackage),
		chosen:     make(map[string]requirement),
	}
}

// directs is sorted, so that a conflict is reported the same way every time.
func (r *resolver) directs(pinned bool) []resolveItem {
	names := make([]string, 0, len(r.require))
	for name := range r.require {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]resolveItem, len(names))
	for i, name := range names {
		items[i] = resolveItem{name: name, req: requirement{constraint: r.require[name]}, pinned: pinned}
	}
	return items
}

func (r *resolver) visit(queue []resolveItem) error {
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if pkg, seen := r.packages[item.name]; seen {
			if err := r.check(item.name, pkg.Version, item.req); err != nil {
				return err
			}
			continue
		}

		// A requirement of the manifest decides the version even when a
		// dependency reaches the package first; the dependency must then be
		// satisfied by it.
		req := item.req
		if constraint, direct := r.require[item.name]; direct {
			req = requirement{constraint: constraint}
		}
		pkg, err := r.lock(item.name, req, item.pinned)
		if err != nil {
			return err
		}
		r.packages[item.name] = pkg
		r.chosen[item.name] = req
		if err := r.check(item.name, pkg.Version, item.req); err != nil {
			return err
		}

		path := append(append([]string(nil), req.path...), item.name)
		for _, dep := range pkg.Depends {
			if r.odooSeries == "" {
				// Without a series there is nothing to check a dependency
				// against; one the manifest requires is resolved by that.
				if _, direct := r.require[dep]; direct {
					continue
				}
				return fmt.Errorf("cannot resolve transitive dependency %q without an Odoo series; set \"odoo_series\" in the manifest", dep)
			}
			queue = append(queue, resolveItem{name: dep, req: requirement{constraint: r.odooSeries, path: path}, pinned: item.pinned})
		}
	}
	return nil
}

func (r *resolver) lock(name string, req requirement, pinned bool) (LockedPackage, error) {
	_, direct := r.require[name]
	if pkg, ok := r.locked[name]; ok && pinned && r.keeps(pkg, req, direct) {
		pkg.Direct = direct
		// keeps found the index, so this replaces the name an older lockfile
		// holds with its URL.
		pkg.IndexURL, _ = index.LockedRegistry(r.indexes, pkg.LockedTo())
		pkg.Index = ""
		return pkg, nil
	}

	lookup, err := index.LookupLocked(r.indexes, r.locked[name].LockedTo(), name, req.constraint, r.odooSeries)
	if err != nil {
		return LockedPackage{}, err
	}

	var deps, external []string
	for _, dep := range lookup.Depends {
		switch dep.Access {
		case "forbidden":
			return LockedPackage{}, fmt.Errorf("%s requires %q (module %s) which you cannot access on the registry", lookup.Name, dep.Package, dep.Module)
		case "external":
			external = append(external, dep.Module)
		case "ok":
			if dep.Package != "" {
				deps = append(deps, dep.Package)
			}
		}
	}
	return lockPackage(lookup, direct, deps, external)
}

// keeps reports whether a locked package can stay as it is. A required
// package locked without the direct flag was written by an update that
// dropped direct and depends, so its depends cannot be trusted and it is
// resolved again. So is a package from an index the manifest no longer has.
func (r *resolver) keeps(pkg LockedPackage, req requirement, direct bool) bool {
	if direct && !pkg.Direct || !satisfies(req.constraint, pkg.Version) || pkg.Dist.CheckIntegrity() != nil {
		return false
	}
	if _, ok := index.LockedRegistry(r.indexes, pkg.LockedTo()); !ok {
		return false
	}
	v, err := version.Parse(pkg.Version)
	if err != nil {
		return false
	}
	return r.odooSeries == "" || !v.IsRelease() || v.SeriesString() == r.odooSeries
}

func (r *resolver) check(name, locked string, req requirement) error {
	if satisfies(req.constraint, locked) {
		return nil
	}
	chosen := r.chosen[name]
	return fmt.Errorf("conflicting requirements for %s: %s requires %s and resolved %s, but %s requires %s", name, chosen, chosen.constraint, locked, req, req.constraint)
}

func (r *resolver) lockFile() (LockFile, error) {
	if err := CheckModuleCollisions(r.packages); err != nil {
		return LockFile{}, err
	}
	hash, err := ComputeHash(r.require, r.indexes, r.odooSeries, r.packages)
	if err != nil {
		return LockFile{}, err
	}
	return LockFile{
		ContentHash: hash,
		Packages:    r.packages,
	}, nil
}

// satisfies reports whether the version v meets constraint. A series accepts
// a dev branch too, like the index does when it checks a version against the
// series of the project: a dependency is on a module, not on a version, so a
// branch required by the manifest serves the packages that depend on it.
func satisfies(constraint, v string) bool {
	c, err := version.ParseConstraint(constraint)
	if err != nil {
		return false
	}
	parsed, err := version.Parse(v)
	if err != nil {
		return false
	}
	if c.Matches(parsed) {
		return true
	}
	isSeries := strings.Count(constraint, ".") == 1 && !strings.HasPrefix(constraint, "dev-")
	return isSeries && parsed.IsDev()
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
			Digest:    lookup.Shasum,
		},
		Direct:          direct,
		Depends:         deps,
		ExternalModules: external,
		IndexURL:        lookup.Registry,
	}
	if err := pkg.Dist.CheckIntegrity(); err != nil {
		return LockedPackage{}, fmt.Errorf("%s %s: %w", lookup.Name, lookup.Version, err)
	}
	return pkg, nil
}

// CheckIntegrity reports whether d pins its content: a zip by its sha256
// digest, a git package by a full commit id.
func (d Dist) CheckIntegrity() error {
	switch d.Type {
	case "zip":
		if d.Digest == "" {
			return fmt.Errorf("zip has no digest to verify it against")
		}
		_, err := digest.Parse(d.Digest)
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
