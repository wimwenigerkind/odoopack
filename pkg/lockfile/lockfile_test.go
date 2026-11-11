package lockfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadOrNewWithoutLockfile(t *testing.T) {
	viper.Set("lock", filepath.Join(t.TempDir(), "odoopack.lock"))
	t.Cleanup(viper.Reset)

	lf, err := LoadOrNew()
	if err != nil {
		t.Fatal(err)
	}
	if lf.Packages == nil || len(lf.Packages) != 0 {
		t.Fatalf("got packages %v, want an empty map", lf.Packages)
	}
}

func TestLoadOrNewRejectsInvalidName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "odoopack.lock")
	viper.Set("lock", path)
	t.Cleanup(viper.Reset)
	if err := os.WriteFile(path, []byte(`{"packages":{"../evil":{"version":"19.0.1.0.0","dist":{"type":"git","url":"https://example.com/x.git"}}}}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadOrNew()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "../evil") {
		t.Fatalf("got error %v, want one naming %s and the invalid name", err, path)
	}
}
