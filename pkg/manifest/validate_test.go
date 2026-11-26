package manifest

import (
	"os"
	"strings"
	"testing"
)

func validManifest() Manifest {
	return Manifest{
		Name:       "test",
		OdooSeries: "19.0",
		Require:    Requirements{"acme/health-check": "19.0"},
		Indexes:    Indexes{{Name: "company", Url: "https://registry.example.com/odoo", Type: "registry"}},
		AddonsDir:  "addons/odoopack",
	}
}

func TestValidateAcceptsValidManifest(t *testing.T) {
	m := validManifest()
	m.Require["acme/base-tools"] = "19.0.1.0.0"
	m.Require["acme/wip"] = "dev-19.0"
	m.Require["acme/feature"] = "dev-feature/x"
	m.Indexes = append(m.Indexes, Index{Name: "local_2", Url: "http://localhost:8080", Type: "registry"})
	m.AddonsDir = "vendor/Odoo.addons/third_party-1"
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	m.Name = ""
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Manifest)
		want   string
	}{
		{"missing series", func(m *Manifest) { m.OdooSeries = "" }, "odoo_series is not set"},
		{"invalid series", func(m *Manifest) { m.OdooSeries = "19" }, `odoo_series "19"`},
		{"leading zero", func(m *Manifest) { m.OdooSeries = "019.0" }, `odoo_series "019.0"`},
		{"latest", func(m *Manifest) { m.Require["acme/health-check"] = "latest" }, "require acme/health-check"},
		{"spaces", func(m *Manifest) { m.Require["acme/health-check"] = " 19.0" }, "surrounding spaces"},
		{"package name", func(m *Manifest) { m.Require["Acme/x"] = "19.0" }, `"Acme/x"`},
		{"index name", func(m *Manifest) { m.Indexes[0].Name = "Has Space/../x" }, `index name "Has Space/../x"`},
		{"index type", func(m *Manifest) { m.Indexes[0].Type = "nonsense" }, "upgrade odoopack"},
		{"credentials", func(m *Manifest) { m.Indexes[0].Url = "https://user:secret@registry.example.com" }, "must not contain credentials"},
		{"query", func(m *Manifest) { m.Indexes[0].Url = "https://registry.example.com?token=x" }, "query or fragment"},
		{"fragment", func(m *Manifest) { m.Indexes[0].Url = "https://registry.example.com#x" }, "query or fragment"},
		{"scheme", func(m *Manifest) { m.Indexes[0].Url = "ftp://registry.example.com" }, "https://"},
		{"no host", func(m *Manifest) { m.Indexes[0].Url = "https:///odoo" }, "has no host"},
		{"upper case host", func(m *Manifest) { m.Indexes[0].Url = "https://Registry.example.com/odoo" }, `must be written as "https://registry.example.com/odoo"`},
		{"trailing slash", func(m *Manifest) { m.Indexes[0].Url = "https://registry.example.com/odoo/" }, `must be written as "https://registry.example.com/odoo"`},
		{"default port", func(m *Manifest) { m.Indexes[0].Url = "https://registry.example.com:443/odoo" }, `must be written as "https://registry.example.com/odoo"`},
		{"percent escape", func(m *Manifest) { m.Indexes[0].Url = "https://registry.example.com/a%2Fb" }, "percent escapes"},
		{"same url twice", func(m *Manifest) {
			m.Indexes = append(m.Indexes, Index{Name: "other", Url: m.Indexes[0].Url, Type: "registry"})
		}, "have the same url"},
		{"release of another series", func(m *Manifest) { m.Require["acme/x"] = "18.0.1.0.0" }, "18.0.1.0.0 targets Odoo 18.0, but odoo_series is 19.0"},
		{"another series", func(m *Manifest) { m.Require["acme/x"] = "18.0" }, "series 18.0 does not match odoo_series 19.0"},
		{"branch of another series", func(m *Manifest) { m.Require["acme/x"] = "dev-18.0" }, "dev-18.0 targets Odoo 18.0"},
		{"name with spaces", func(m *Manifest) { m.Name = "My Project" }, `name "My Project" must be written like a package name`},
		{"addons_dir missing", func(m *Manifest) { m.AddonsDir = "" }, "addons_dir is not set"},
		{"addons_dir backslash", func(m *Manifest) { m.AddonsDir = `addons\odoopack` }, "addons_dir"},
		{"addons_dir absolute", func(m *Manifest) { m.AddonsDir = "/srv/addons" }, "addons_dir"},
		{"addons_dir drive", func(m *Manifest) { m.AddonsDir = "C:/addons" }, "addons_dir"},
		{"addons_dir parent", func(m *Manifest) { m.AddonsDir = "../addons" }, `must not contain ".."`},
		{"addons_dir dot", func(m *Manifest) { m.AddonsDir = "." }, `must not contain "."`},
		{"addons_dir inner dot", func(m *Manifest) { m.AddonsDir = "addons/./odoopack" }, `must not contain "."`},
		{"addons_dir trailing slash", func(m *Manifest) { m.AddonsDir = "addons/" }, "addons_dir"},
		{"addons_dir space", func(m *Manifest) { m.AddonsDir = "my addons" }, "addons_dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.change(&m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got error %v, want one containing %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error %v leaks the credentials", err)
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	m := validManifest()
	m.OdooSeries = ""
	m.Indexes[0].Type = "nonsense"
	err := m.Validate()
	if err == nil || !strings.Contains(err.Error(), "odoo_series") || !strings.Contains(err.Error(), "nonsense") {
		t.Fatalf("got error %v, want both problems", err)
	}
}

func TestSaveRefusesInvalidManifest(t *testing.T) {
	path := useManifest(t, "")
	m := validManifest()
	m.Indexes[0].Url = "https://user:secret@registry.example.com"

	if err := Save(m); err == nil || !strings.Contains(err.Error(), "refusing to write") {
		t.Fatalf("got error %v, want the write refused", err)
	}
	if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
		t.Fatalf("manifest was written: %q (%v)", data, err)
	}
}

func TestCanonicalURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://Registry.Example.com/":      "https://registry.example.com",
		"HTTPS://registry.example.com/odoo/": "https://registry.example.com/odoo",
		"https://registry.example.com:443":   "https://registry.example.com",
		"http://registry.example.com:80/x":   "http://registry.example.com/x",
		"https://registry.example.com:8443":  "https://registry.example.com:8443",
		"http://localhost:8080//":            "http://localhost:8080",
		"https://[::1]:443/":                 "https://[::1]",
		"https://registry.example.com":       "https://registry.example.com",
	} {
		if got := CanonicalURL(raw); got != want {
			t.Errorf("CanonicalURL(%q) = %q, want %q", raw, got, want)
		}
		if err := ValidIndexURL(want); err != nil {
			t.Errorf("canonical url %q is not valid: %v", want, err)
		}
	}
}

func TestCanonicalURLKeepsPercentEscapes(t *testing.T) {
	const raw = "https://registry.example.com/a%2Fb"
	if got := CanonicalURL(raw); got != raw {
		t.Fatalf("CanonicalURL(%q) = %q, want it unchanged", raw, got)
	}
	if err := ValidIndexURL(raw); err == nil || !strings.Contains(err.Error(), "percent escapes") {
		t.Fatalf("got error %v, want one about percent escapes", err)
	}
}
