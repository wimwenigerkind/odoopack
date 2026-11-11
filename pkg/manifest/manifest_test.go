package manifest

import (
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
