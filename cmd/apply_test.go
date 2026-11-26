package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

// release is a version of an addon whose zipball is dist.
func release(dist lockfile.Dist, version string, depends ...string) registryapi.Version {
	v := registryapi.Version{Version: version, Type: registryapi.TypeZip, URL: dist.URL, Digest: dist.Digest}
	for _, d := range depends {
		v.Depends = append(v.Depends, registryapi.Dep{Module: name.ModuleName(d), Package: d, Access: registryapi.AccessOK})
	}
	return v
}

// serveRegistry serves addons by name and answers 404 for any other.
func serveRegistry(t *testing.T, addons ...registryapi.Addon) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, addon := range addons {
			if r.URL.Path == "/registry/v1/addons/"+addon.Name {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(addon)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func lockedAt(pkg, indexURL string, dist lockfile.Dist, version string, direct bool, depends ...string) lockfile.LockedPackage {
	return lockfile.LockedPackage{Version: version, Module: name.ModuleName(pkg), Dist: dist, Direct: direct, Depends: depends, IndexURL: indexURL}
}

// project writes m and a lockfile with packages into a temporary project
// directory, and installs each package as a directory holding stale.py, which
// an install of the package replaces.
func project(t *testing.T, m *manifest.Manifest, packages map[string]lockfile.LockedPackage) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := manifest.Save(*m); err != nil {
		t.Fatal(err)
	}
	hash, err := lockfile.ComputeHash(m.Require, m.Indexes, m.OdooSeries)
	if err != nil {
		t.Fatal(err)
	}
	if err := lockfile.Save(lockfile.LockFile{ContentHash: hash, Packages: packages}); err != nil {
		t.Fatal(err)
	}
	if err := addonsdir.Claim(m.AddonsDir, nil); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range packages {
		dir := filepath.Join(m.AddonsDir, pkg.Module)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "stale.py"), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// assertAddons checks the directory of each package: "installed" by this run,
// "kept" from before, or "absent".
func assertAddons(t *testing.T, m *manifest.Manifest, want map[string]string) {
	t.Helper()
	for pkg, state := range want {
		dir := filepath.Join(m.AddonsDir, name.ModuleName(pkg))
		_, errStale := os.Stat(filepath.Join(dir, "stale.py"))
		_, errDir := os.Stat(dir)
		var got string
		switch {
		case os.IsNotExist(errDir):
			got = "absent"
		case errStale == nil:
			got = "kept"
		default:
			got = "installed"
		}
		if got != state {
			t.Errorf("%s is %s, want %s", pkg, got, state)
		}
	}
}

func TestRequireInstallsChangedPackages(t *testing.T) {
	dist := addonZip(t)
	url := serveRegistry(t,
		registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{release(dist, "19.0.1.0.0")}},
		registryapi.Addon{Name: "acme/base-tools", Versions: []registryapi.Version{release(dist, "19.0.1.0.0"), release(dist, "19.0.2.0.0")}},
		registryapi.Addon{Name: "acme/other", Versions: []registryapi.Version{release(dist, "19.0.1.0.0")}},
	)
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	m.Indexes = manifest.Indexes{{Name: "a", Url: url, Type: "registry"}}
	m.Require = manifest.Requirements{"acme/base-tools": "19.0", "acme/other": "19.0"}
	project(t, m, map[string]lockfile.LockedPackage{
		"acme/base-tools": lockedAt("acme/base-tools", url, dist, "19.0.1.0.0", true),
		"acme/other":      lockedAt("acme/other", url, dist, "19.0.1.0.0", true),
	})

	requireCmd.Run(requireCmd, []string{"acme/health-check"})

	assertAddons(t, m, map[string]string{
		"acme/health-check": "installed",
		"acme/base-tools":   "installed",
		"acme/other":        "kept",
	})
	lock, err := lockfile.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := lock.Packages["acme/base-tools"].Version; got != "19.0.2.0.0" {
		t.Errorf("acme/base-tools locked at %s, want 19.0.2.0.0", got)
	}
}

func TestUpdateOneResolvesItsTree(t *testing.T) {
	dist := addonZip(t)
	url := serveRegistry(t,
		registryapi.Addon{Name: "acme/health-check", Versions: []registryapi.Version{
			release(dist, "19.0.1.0.0", "acme/old-dep"),
			release(dist, "19.0.2.0.0", "acme/base-tools"),
		}},
		registryapi.Addon{Name: "acme/base-tools", Versions: []registryapi.Version{release(dist, "19.0.1.0.0")}},
		registryapi.Addon{Name: "acme/old-dep", Versions: []registryapi.Version{release(dist, "19.0.1.0.0")}},
		registryapi.Addon{Name: "acme/other", Versions: []registryapi.Version{release(dist, "19.0.1.0.0"), release(dist, "19.0.2.0.0")}},
	)
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	m.Indexes = manifest.Indexes{{Name: "a", Url: url, Type: "registry"}}
	m.Require = manifest.Requirements{"acme/health-check": "19.0", "acme/other": "19.0"}
	project(t, m, map[string]lockfile.LockedPackage{
		"acme/health-check": lockedAt("acme/health-check", url, dist, "19.0.1.0.0", true, "acme/old-dep"),
		"acme/old-dep":      lockedAt("acme/old-dep", url, dist, "19.0.1.0.0", false),
		"acme/other":        lockedAt("acme/other", url, dist, "19.0.1.0.0", true),
	})

	updateCmd.Run(updateCmd, []string{"acme/health-check"})

	assertAddons(t, m, map[string]string{
		"acme/health-check": "installed",
		"acme/base-tools":   "installed",
		"acme/old-dep":      "absent",
		"acme/other":        "kept",
	})
	lock, err := lockfile.Load()
	if err != nil {
		t.Fatal(err)
	}
	hc := lock.Packages["acme/health-check"]
	if hc.Version != "19.0.2.0.0" || !hc.Direct || strings.Join(hc.Depends, ",") != "acme/base-tools" {
		t.Errorf("acme/health-check locked as %+v", hc)
	}
	if got := lock.Packages["acme/other"].Version; got != "19.0.1.0.0" {
		t.Errorf("acme/other locked at %s, want it kept at 19.0.1.0.0", got)
	}
}

func TestRemoveRemovesOrphans(t *testing.T) {
	dist := addonZip(t)
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	// Every package stays locked as it is, so remove must not ask the index.
	url := "http://127.0.0.1:1"
	m.Indexes = manifest.Indexes{{Name: "a", Url: url, Type: "registry"}}
	m.Require = manifest.Requirements{"acme/health-check": "19.0", "acme/other": "19.0"}
	project(t, m, map[string]lockfile.LockedPackage{
		"acme/health-check": lockedAt("acme/health-check", url, dist, "19.0.1.0.0", true, "acme/base-tools", "acme/shared"),
		"acme/base-tools":   lockedAt("acme/base-tools", url, dist, "19.0.1.0.0", false),
		"acme/other":        lockedAt("acme/other", url, dist, "19.0.1.0.0", true, "acme/shared"),
		"acme/shared":       lockedAt("acme/shared", url, dist, "19.0.1.0.0", false),
	})

	removeCmd.Run(removeCmd, []string{"acme/health-check"})

	assertAddons(t, m, map[string]string{
		"acme/health-check": "absent",
		"acme/base-tools":   "absent",
		"acme/other":        "kept",
		"acme/shared":       "kept",
	})
	lock, err := lockfile.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 2 {
		t.Errorf("locked %v, want acme/other and acme/shared", lock.Packages)
	}
	saved, err := manifest.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.Require["acme/health-check"]; ok || len(saved.Require) != 1 {
		t.Errorf("manifest requires %v", saved.Require)
	}
}

// runIfChild runs c in a child process started by installInChild.
func runIfChild(c *cobra.Command, args ...string) bool {
	if os.Getenv("ODOOPACK_TEST_INSTALL") != "1" {
		return false
	}
	c.Run(c, args)
	return true
}

func TestRemoveRefusesWhileDependedOn(t *testing.T) {
	if runIfChild(removeCmd, "acme/base-tools") {
		return
	}

	zipDigest := "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543"
	dist := lockfile.Dist{Type: "zip", URL: "https://example.com/x.zip", Digest: zipDigest}
	url := "http://127.0.0.1:1"
	lock, err := json.Marshal(lockfile.LockFile{FormatVersion: lockfile.FormatVersion, ContentHash: "sha256:00", Packages: map[string]lockfile.LockedPackage{
		"acme/health-check": lockedAt("acme/health-check", url, dist, "19.0.1.0.0", true, "acme/base-tools"),
		"acme/base-tools":   lockedAt("acme/base-tools", url, dist, "19.0.1.0.0", true),
		"acme/other":        lockedAt("acme/other", url, dist, "19.0.1.0.0", true, "acme/base-tools"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, out := installInChild(t, map[string]string{
		"odoopack.json": `{"name":"test","odoo_series":"19.0","require":{"acme/health-check":"19.0","acme/base-tools":"19.0","acme/other":"19.0"},"indexes":[{"name":"a","url":"http://127.0.0.1:1","type":"registry"}],"addons_dir":"addons/odoopack"}`,
		"odoopack.lock": string(lock),
		filepath.Join("addons", "odoopack", addonsdir.MarkerName):                 "",
		filepath.Join("addons", "odoopack", "acme_base_tools", "__manifest__.py"): "{}",
	})
	if !strings.Contains(out, "cannot remove acme/base-tools: acme/health-check, acme/other depend(s) on it") {
		t.Fatalf("output does not name the dependents:\n%s", out)
	}
}

func TestApplyLockMovesChangedModule(t *testing.T) {
	dist := addonZip(t)
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	m.Require = manifest.Requirements{"acme/health-check": "19.0"}
	before := lockedAt("acme/health-check", "https://registry.example.com", dist, "19.0.1.0.0", true)
	before.Module = "health_check"
	project(t, m, map[string]lockfile.LockedPackage{"acme/health-check": before})
	after := before
	after.Module = "acme_health_check"

	installed, removed, err := applyLock(m,
		lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{"acme/health-check": before}},
		lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{"acme/health-check": after}})
	if err != nil {
		t.Fatal(err)
	}
	if installed != 1 || removed != 1 {
		t.Errorf("installed %d and removed %d, want 1 and 1", installed, removed)
	}
	if _, err := os.Stat(filepath.Join(m.AddonsDir, "health_check")); !os.IsNotExist(err) {
		t.Errorf("the old module directory is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.AddonsDir, "acme_health_check", "__manifest__.py")); err != nil {
		t.Errorf("the package is not installed in its new module: %v", err)
	}
}
