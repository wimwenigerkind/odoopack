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
	if err := os.WriteFile(path, []byte(`{"name":"test","odoo_series":"19.0","require":{"acme/../evil":"19.0"},"addons_dir":"addons/odoopack"}`), 0644); err != nil {
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
	useManifest(t, `{"name":"test","odoo_series":"19.0","addons_dir":"addons/odoopack"}`)

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if m.FormatVersion != 1 {
		t.Fatalf("got format_version %d, want 1", m.FormatVersion)
	}
}

func TestLoadRejectsNewerFormatVersion(t *testing.T) {
	path := useManifest(t, fmt.Sprintf(`{"format_version":%d,"name":"test","odoo_series":"19.0","addons_dir":"addons/odoopack"}`, FormatVersion+1))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "upgrade odoopack") {
		t.Fatalf("got error %v, want one naming %s and asking to upgrade", err, path)
	}
}

func TestSaveWritesFormatVersionFirst(t *testing.T) {
	path := useManifest(t, `{"name":"test","odoo_series":"19.0","addons_dir":"addons/odoopack"}`)

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

func TestSaveKeepsUnknownKeys(t *testing.T) {
	path := useManifest(t, `{
  "zeta": {"b": 1, "a": [true]},
  "$schema": "https://example.com/manifest.json",
  "format_version": 1,
  "name": "test",
  "description": "kept",
  "odoo_series": "19.0",
  "require": {},
  "indexes": [{"priority": 10, "name": "company", "url": "https://registry.example.com", "type": "registry"}],
  "addons_dir": "addons/odoopack",
  "extra": {"acme": {"team": "core"}}
}`)

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(m.UnknownKeys(), ","), "description,indexes.company.priority,zeta"; got != want {
		t.Fatalf("got unknown keys %s, want %s", got, want)
	}
	if err := Save(*m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := `{
  "$schema": "https://example.com/manifest.json",
  "format_version": 1,
  "name": "test",
  "odoo_series": "19.0",
  "require": {},
  "indexes": [
    {
      "name": "company",
      "url": "https://registry.example.com",
      "type": "registry",
      "priority": 10
    }
  ],
  "addons_dir": "addons/odoopack",
  "extra": {
    "acme": {
      "team": "core"
    }
  },
  "description": "kept",
  "zeta": {
    "b": 1,
    "a": [
      true
    ]
  }
}`
	if string(data) != want {
		t.Fatalf("got\n%s\nwant\n%s", data, want)
	}
}

func TestLoadRejectsExtraThatIsNotAnObject(t *testing.T) {
	useManifest(t, `{"name":"test","odoo_series":"19.0","addons_dir":"addons/odoopack","extra":[1]}`)

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "extra must be an object") {
		t.Fatalf("got error %v, want one about extra", err)
	}
}

func TestLoadRejectsDuplicateKeys(t *testing.T) {
	for _, content := range []string{
		`{"name":"test","odoo_series":"19.0","odoo_series":"18.0","addons_dir":"addons/odoopack"}`,
		`{"name":"test","odoo_series":"19.0","require":{"acme/x":"19.0","acme/x":"dev-19.0"},"addons_dir":"addons/odoopack"}`,
	} {
		useManifest(t, content)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("%s: got error %v, want one about the duplicate key", content, err)
		}
	}
}

func TestLoadMatchesKeysExactly(t *testing.T) {
	useManifest(t, `{"name":"test","odoo_series":"19.0","Require":{"acme/x":"19.0"},"addons_dir":"addons/odoopack"}`)

	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Require) != 0 || strings.Join(m.UnknownKeys(), ",") != "Require" {
		t.Fatalf("got require %v and unknown keys %v, want Require kept as an unknown key", m.Require, m.UnknownKeys())
	}
}

func TestSaveDoesNotEscapeHTML(t *testing.T) {
	path := useManifest(t, `{"name":"a&b","odoo_series":"19.0","addons_dir":"addons/odoopack","extra":{"note":"<x>"}}`)

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
	if !bytes.Contains(data, []byte(`"a&b"`)) || !bytes.Contains(data, []byte(`"<x>"`)) {
		t.Fatalf("manifest escapes HTML:\n%s", data)
	}
}
