package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

func TestInstallAllRefusesForeignDir(t *testing.T) {
	t.Chdir(t.TempDir())
	own := filepath.Join("addons", "my_module", "__manifest__.py")
	if err := os.MkdirAll(filepath.Dir(own), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	m := manifest.NewManifest("test", "addons", "19.0")
	if err := installAll(m, lockfile.LockFile{}, lockfile.LockFile{}); err == nil {
		t.Fatal("installAll cleared a directory without marker")
	}
	if _, err := os.Stat(own); err != nil {
		t.Fatalf("foreign file was removed: %v", err)
	}
}

// addonRepo creates a local git repository holding an addon.
func addonRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "__manifest__.py"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestInstallAllReplacesPreviousInstall(t *testing.T) {
	repo := addonRepo(t)
	t.Chdir(t.TempDir())
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	if err := addonsdir.Claim(m.AddonsPath, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(m.AddonsPath, "old_addon"), 0755); err != nil {
		t.Fatal(err)
	}

	lock := lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{
		"new-addon": {Dist: lockfile.Dist{Type: "git", URL: repo}},
	}}
	if err := installAll(m, lock, lockfile.LockFile{}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(m.AddonsPath, "old_addon")); !os.IsNotExist(err) {
		t.Errorf("old_addon still exists: %v", err)
	}
	for _, name := range []string{addonsdir.MarkerName, filepath.Join("new_addon", "__manifest__.py")} {
		if _, err := os.Stat(filepath.Join(m.AddonsPath, name)); err != nil {
			t.Errorf("%s missing after install: %v", name, err)
		}
	}
}

func TestInstallAllAdoptsLegacyInstall(t *testing.T) {
	repo := addonRepo(t)
	t.Chdir(t.TempDir())
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	legacy := filepath.Join(m.AddonsPath, "new_addon", "legacy.py")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, nil, 0644); err != nil {
		t.Fatal(err)
	}

	lock := lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{
		"new-addon": {Dist: lockfile.Dist{Type: "git", URL: repo}},
	}}
	if err := installAll(m, lock, lockfile.LockFile{}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy file still exists: %v", err)
	}
	for _, name := range []string{addonsdir.MarkerName, filepath.Join("new_addon", "__manifest__.py")} {
		if _, err := os.Stat(filepath.Join(m.AddonsPath, name)); err != nil {
			t.Errorf("%s missing after install: %v", name, err)
		}
	}
}

func TestInstallAllAdoptsLegacyInstallAfterResolvingAgain(t *testing.T) {
	repo := addonRepo(t)
	t.Chdir(t.TempDir())
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	dropped := filepath.Join(m.AddonsPath, "dropped_addon", "__init__.py")
	if err := os.MkdirAll(filepath.Dir(dropped), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dropped, nil, 0644); err != nil {
		t.Fatal(err)
	}

	previous := lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{
		"dropped-addon": {Dist: lockfile.Dist{Type: "git", URL: repo}},
	}}
	lock := lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{
		"new-addon": {Dist: lockfile.Dist{Type: "git", URL: repo}},
	}}
	if err := installAll(m, lock, previous); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Dir(dropped)); !os.IsNotExist(err) {
		t.Errorf("dropped_addon still exists: %v", err)
	}
	for _, name := range []string{addonsdir.MarkerName, filepath.Join("new_addon", "__manifest__.py")} {
		if _, err := os.Stat(filepath.Join(m.AddonsPath, name)); err != nil {
			t.Errorf("%s missing after install: %v", name, err)
		}
	}
}

// install exits the process on an error, so the test runs it in a child
// process of the test binary.
func TestInstallRejectsInvalidLockfileName(t *testing.T) {
	if os.Getenv("ODOOPACK_TEST_INSTALL") == "1" {
		installCmd.Run(installCmd, nil)
		return
	}

	dir := t.TempDir()
	files := map[string]string{
		"odoopack.json": `{"name":"test","odoo":"19.0","require":{"acme/health-check":"19.0"},"addons_path":"addons/odoopack"}`,
		"odoopack.lock": `{"content_hash":"sha256:00","packages":{"../evil":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`,
		filepath.Join("addons", "odoopack", addonsdir.MarkerName):                   "",
		filepath.Join("addons", "odoopack", "acme_health_check", "__manifest__.py"): "{}",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestInstallRejectsInvalidLockfileName$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ODOOPACK_TEST_INSTALL=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install succeeded:\n%s", out)
	}
	if !strings.Contains(string(out), "odoopack.lock") || !strings.Contains(string(out), "../evil") {
		t.Fatalf("output does not name the lockfile and the invalid name:\n%s", out)
	}

	for name, content := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != content {
			t.Errorf("%s changed: %q, %v", name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); !os.IsNotExist(err) {
		t.Errorf("evil exists: %v", err)
	}
}
