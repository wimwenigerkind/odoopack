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
	m.Indexes = append(m.Indexes, Index{Name: "local_2", Url: "http://localhost:8080", Type: "registry"})
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
