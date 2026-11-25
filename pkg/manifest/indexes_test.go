package manifest

import (
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func names(ix Indexes) string {
	var list []string
	for _, idx := range ix {
		list = append(list, idx.Name+"="+idx.Url)
	}
	return strings.Join(list, ",")
}

func TestLoadMigratesIndexObject(t *testing.T) {
	const public = "https://odoopack.example.com"
	tests := []struct {
		name    string
		indexes string
		want    string
	}{
		{"sorted, default appended", `"indexes":{"zeta":{"url":"https://z.example.com","type":"registry"},"alpha":{"url":"https://a.example.com","type":"registry"}},`, "alpha=https://a.example.com,zeta=https://z.example.com,default=" + public},
		{"empty", `"indexes":{},`, "default=" + public},
		{"missing", ``, "default=" + public},
		{"named default", `"indexes":{"default":{"url":"https://mirror.example.com","type":"registry"}},`, "default=https://mirror.example.com"},
		{"default url", `"indexes":{"public":{"url":"` + public + `","type":"registry"}},`, "public=" + public},
		{"array", `"indexes":[{"name":"company","url":"https://registry.example.com","type":"registry"}],`, "company=https://registry.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useManifest(t, `{"name":"test",`+tt.indexes+`"odoo_series":"19.0","addons_dir":"addons/odoopack"}`)
			viper.Set("default_index_url", public)

			m, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if got := names(m.Indexes); got != tt.want {
				t.Fatalf("got indexes %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLoadRejectsInvalidIndexNames(t *testing.T) {
	for _, indexes := range []string{
		`[{"url":"https://a.example.com","type":"registry"}]`,
		`[{"name":"a","url":"https://a.example.com","type":"registry"},{"name":"a","url":"https://b.example.com","type":"registry"}]`,
	} {
		useManifest(t, `{"name":"test","odoo_series":"19.0","indexes":`+indexes+`,"addons_dir":"addons/odoopack"}`)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "index name") {
			t.Errorf("indexes %s: got error %v, want one about the index names", indexes, err)
		}
	}
}

func TestInitWritesDefaultIndex(t *testing.T) {
	path := useManifest(t, "")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	viper.Set("default_index_url", "https://odoopack.example.com")

	if _, err := Init("test", "19.0"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var written struct {
		Indexes []map[string]string `json:"indexes"`
	}
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatal(err)
	}
	if len(written.Indexes) != 1 || written.Indexes[0]["name"] != DefaultIndexName || written.Indexes[0]["url"] != "https://odoopack.example.com" {
		t.Fatalf("got indexes %v, want the default registry", written.Indexes)
	}
}

func TestIndexesAddAndRemove(t *testing.T) {
	ix := Indexes{{Name: "default", Url: "https://public.example.com"}}

	ix = ix.Add(Index{Name: "company", Url: "https://company.example.com"}, false)
	ix = ix.Add(Index{Name: "mirror", Url: "https://mirror.example.com"}, true)
	ix = ix.Add(Index{Name: "default", Url: "https://other.example.com"}, false)
	if got, want := names(ix), "company=https://company.example.com,default=https://other.example.com,mirror=https://mirror.example.com"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	ix, removed := ix.Remove("default")
	if !removed || names(ix) != "company=https://company.example.com,mirror=https://mirror.example.com" {
		t.Fatalf("got %s (removed %v) after removing default", names(ix), removed)
	}
	if _, removed := ix.Remove("missing"); removed {
		t.Fatal("removed an index that does not exist")
	}
}
