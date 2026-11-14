package cmd

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
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

// addonZip serves a zipball holding an addon and returns its dist. A git
// package would need an https or ssh server, the only transports the git
// installer allows.
func addonZip(t *testing.T) lockfile.Dist {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("addon-1.0/__manifest__.py")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	body := buf.Bytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	viper.Set("no_cache", true)
	t.Cleanup(viper.Reset)
	return lockfile.Dist{Type: "zip", URL: srv.URL, Shasum: fmt.Sprintf("sha256:%x", sha256.Sum256(body))}
}

func TestInstallAllReplacesPreviousInstall(t *testing.T) {
	dist := addonZip(t)
	t.Chdir(t.TempDir())
	m := manifest.NewManifest("test", "addons/odoopack", "19.0")
	if err := addonsdir.Claim(m.AddonsPath, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(m.AddonsPath, "old_addon"), 0755); err != nil {
		t.Fatal(err)
	}

	lock := lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{
		"new-addon": {Dist: dist},
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
	dist := addonZip(t)
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
		"new-addon": {Dist: dist},
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
	dist := addonZip(t)
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
		"dropped-addon": {Dist: dist},
	}}
	lock := lockfile.LockFile{Packages: map[string]lockfile.LockedPackage{
		"new-addon": {Dist: dist},
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

// installInChild runs install in a child process of the test binary, since
// install exits the process on an error. The calling test must start with
// runInstallIfChild. It writes files into a temporary project directory and
// fails the test if install succeeds or changes any of them.
func installInChild(t *testing.T, files map[string]string) (dir, out string) {
	t.Helper()
	dir = t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ODOOPACK_TEST_INSTALL=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("install succeeded:\n%s", output)
	}

	for name, content := range files {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != content {
			t.Errorf("%s changed: %q, %v", name, got, err)
		}
	}
	return dir, string(output)
}

func runInstallIfChild() bool {
	if os.Getenv("ODOOPACK_TEST_INSTALL") != "1" {
		return false
	}
	installCmd.Run(installCmd, nil)
	return true
}

func TestInstallRejectsInvalidLockfileName(t *testing.T) {
	if runInstallIfChild() {
		return
	}

	dir, out := installInChild(t, map[string]string{
		"odoopack.json": `{"name":"test","odoo":"19.0","require":{"acme/health-check":"19.0"},"addons_path":"addons/odoopack"}`,
		"odoopack.lock": `{"content_hash":"sha256:00","packages":{"../evil":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`,
		filepath.Join("addons", "odoopack", addonsdir.MarkerName):                   "",
		filepath.Join("addons", "odoopack", "acme_health_check", "__manifest__.py"): "{}",
	})
	if !strings.Contains(out, "odoopack.lock") || !strings.Contains(out, "../evil") {
		t.Fatalf("output does not name the lockfile and the invalid name:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); !os.IsNotExist(err) {
		t.Errorf("evil exists: %v", err)
	}
}

func TestInstallRejectsNewerLockfileFormat(t *testing.T) {
	if runInstallIfChild() {
		return
	}

	// The content hash is stale on purpose: install must stop before it would
	// resolve again and overwrite the lockfile.
	_, out := installInChild(t, map[string]string{
		"odoopack.json": `{"name":"test","odoo":"19.0","require":{"acme/health-check":"19.0"},"addons_path":"addons/odoopack"}`,
		"odoopack.lock": `{"format":2,"content_hash":"sha256:00","packages":{"acme/health-check":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`,
		filepath.Join("addons", "odoopack", addonsdir.MarkerName):                   "",
		filepath.Join("addons", "odoopack", "acme_health_check", "__manifest__.py"): "{}",
	})
	if !strings.Contains(out, "odoopack.lock") || !strings.Contains(out, "upgrade odoopack") {
		t.Fatalf("output does not name the lockfile and ask to upgrade:\n%s", out)
	}
}
