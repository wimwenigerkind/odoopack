package manifest

import (
	"encoding/json/jsontext"
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

func TestIndexesAddKeepsUnknownKeys(t *testing.T) {
	ix := Indexes{{Name: "company", Url: "https://old.example.com", Type: "registry", Unknown: map[string]jsontext.Value{"only": jsontext.Value(`["acme/*"]`)}}}

	ix = ix.Add(Index{Name: "company", Url: "https://new.example.com", Type: "registry"}, false)

	if len(ix) != 1 || ix[0].Url != "https://new.example.com" || string(ix[0].Unknown["only"]) != `["acme/*"]` {
		t.Fatalf("got %+v, want the new url with the unknown key kept", ix)
	}
}
