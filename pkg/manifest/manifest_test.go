package manifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadRejectsInvalidName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odoopack.json")
	viper.Set("manifest", path)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(`{"name":"test","require":{"acme/../evil":"19.0"},"addons_path":"addons/odoopack"}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "acme/../evil") {
		t.Fatalf("got error %v, want one naming %s and the invalid name", err, path)
	}
}

// useManifest points the manifest setting at a manifest in a temporary
// directory that holds content.
func useManifest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "odoopack.json")
	viper.Set("manifest", path)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadWithoutFormatVersionIsOne(t *testing.T) {
	useManifest(t, `{"name":"test","odoo":"19.0","addons_path":"addons/odoopack"}`)

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if m.FormatVersion != 1 {
		t.Fatalf("got format_version %d, want 1", m.FormatVersion)
	}
}

func TestLoadRejectsNewerFormatVersion(t *testing.T) {
	path := useManifest(t, fmt.Sprintf(`{"format_version":%d,"name":"test","odoo":"19.0","addons_path":"addons/odoopack"}`, FormatVersion+1))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "upgrade odoopack") {
		t.Fatalf("got error %v, want one naming %s and asking to upgrade", err, path)
	}
}

func TestSaveWritesFormatVersionFirst(t *testing.T) {
	path := useManifest(t, `{"name":"test","odoo":"19.0","addons_path":"addons/odoopack"}`)

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(*m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("{\n  \"format_version\": %d,\n", FormatVersion); !bytes.HasPrefix(data, []byte(want)) {
		t.Fatalf("manifest does not start with %q:\n%s", want, data)
	}
}

func TestLoadReadsLegacyKeys(t *testing.T) {
	path := useManifest(t, `{"name":"test","odoo":"19.0","addons_path":"addons/custom"}`)

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if m.OdooSeries != "19.0" || m.AddonsDir != "addons/custom" {
		t.Fatalf("got odoo_series %q and addons_dir %q, want the values of odoo and addons_path", m.OdooSeries, m.AddonsDir)
	}
	if err := Save(*m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"odoo":`, `"addons_path":`} {
		if bytes.Contains(data, []byte(key)) {
			t.Errorf("manifest still has the legacy key %s:\n%s", key, data)
		}
	}
	for _, key := range []string{`"odoo_series": "19.0"`, `"addons_dir": "addons/custom"`} {
		if !bytes.Contains(data, []byte(key)) {
			t.Errorf("manifest lacks %s:\n%s", key, data)
		}
	}
}
