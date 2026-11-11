package addonsdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(project, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(project, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, "addons"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("addons", filepath.Join(project, "inner")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		ok   bool
	}{
		{"addons/odoopack", true},
		{"addons", true},
		{"inner/odoopack", true},
		{"./addons/../vendor", true},
		{"", false},
		{"./", false},
		{".", false},
		{"..", false},
		{"/", false},
		{outside, false},
		{"a/../..", false},
		{"addons/..", false},
		{"linked", false},
		{"linked/odoopack", false},
		{"dangling/odoopack", false},
	}
	for _, tt := range tests {
		err := Check(project, tt.path)
		if tt.ok && err != nil {
			t.Errorf("Check(%q) = %v, want nil", tt.path, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("Check(%q) = nil, want an error", tt.path)
		}
	}
}

func TestClearRefusesForeignDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "addons")
	own := filepath.Join(dir, "my_module", "__manifest__.py")
	if err := os.MkdirAll(filepath.Dir(own), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Clear(dir, nil); err == nil {
		t.Fatal("Clear succeeded on a directory without marker")
	}
	if err := Claim(dir, nil); err == nil {
		t.Fatal("Claim succeeded on a directory without marker")
	}
	if err := RemoveAddon(dir, "my_module", nil); err == nil {
		t.Fatal("RemoveAddon succeeded in a directory without marker")
	}
	if _, err := os.Stat(own); err != nil {
		t.Fatalf("foreign file was touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, MarkerName)); err == nil {
		t.Fatal("marker was written into a foreign directory")
	}
}

func TestClearEmptiesOwnedDir(t *testing.T) {
	for _, name := range []string{"missing", "empty", "owned"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "addons")
			switch name {
			case "empty":
				if err := os.Mkdir(dir, 0755); err != nil {
					t.Fatal(err)
				}
			case "owned":
				if err := Claim(dir, nil); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, "old_addon"), 0755); err != nil {
					t.Fatal(err)
				}
			}

			if err := Clear(dir, nil); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != MarkerName {
				t.Fatalf("entries after Clear = %v, want only %s", entries, MarkerName)
			}
		})
	}
}

func TestRemoveAddonRejectsNames(t *testing.T) {
	dir := t.TempDir()
	if err := Claim(dir, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "a/b", "/abs"} {
		if err := RemoveAddon(dir, name, nil); err == nil {
			t.Errorf("RemoveAddon(%q) = nil, want an error", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, MarkerName)); err != nil {
		t.Fatalf("addons_path was touched: %v", err)
	}
}

// legacyInstall fills dir like an odoopack version before the marker did.
func legacyInstall(t *testing.T, dir string, addons ...string) {
	t.Helper()
	for _, addon := range addons {
		if err := os.MkdirAll(filepath.Join(dir, addon), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, addon, "__manifest__.py"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAdoptLegacyInstall(t *testing.T) {
	locked := map[string]bool{"sale_extra": true, "stock_extra": true, "not_installed": true}

	cleared := filepath.Join(t.TempDir(), "addons")
	legacyInstall(t, cleared, "sale_extra", "stock_extra")
	if err := Clear(cleared, locked); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cleared)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != MarkerName {
		t.Fatalf("entries after Clear = %v, want only %s", entries, MarkerName)
	}

	removed := filepath.Join(t.TempDir(), "addons")
	legacyInstall(t, removed, "sale_extra", "stock_extra")
	if err := RemoveAddon(removed, "sale_extra", locked); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{MarkerName: true, "stock_extra": true, "sale_extra": false} {
		if _, err := os.Lstat(filepath.Join(removed, name)); (err == nil) != want {
			t.Errorf("%s exists = %v, want %v", name, err == nil, want)
		}
	}
}

func TestAdoptRefusesForeignEntries(t *testing.T) {
	locked := map[string]bool{"sale_extra": true, "stock_extra": true}
	tests := map[string]func(t *testing.T, dir string){
		"file": func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0644); err != nil {
				t.Fatal(err)
			}
		},
		"unknown directory": func(t *testing.T, dir string) {
			legacyInstall(t, dir, "my_module")
		},
		"symlink named like a locked addon": func(t *testing.T, dir string) {
			if err := os.Symlink(t.TempDir(), filepath.Join(dir, "stock_extra")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, add := range tests {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "addons")
			legacyInstall(t, dir, "sale_extra")
			add(t, dir)

			if err := Clear(dir, locked); err == nil {
				t.Fatal("Clear adopted a directory with foreign content")
			}
			if err := Claim(dir, locked); err == nil {
				t.Fatal("Claim adopted a directory with foreign content")
			}
			if err := RemoveAddon(dir, "sale_extra", locked); err == nil {
				t.Fatal("RemoveAddon adopted a directory with foreign content")
			}
			if _, err := os.Stat(filepath.Join(dir, "sale_extra", "__manifest__.py")); err != nil {
				t.Fatalf("addon was touched: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, MarkerName)); err == nil {
				t.Fatal("marker was written")
			}
		})
	}
}

func TestNoAdoptionWithoutLockfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "addons")
	legacyInstall(t, dir, "sale_extra")
	for _, locked := range []map[string]bool{nil, {}} {
		if err := Clear(dir, locked); err == nil {
			t.Fatalf("Clear(%v) adopted a directory", locked)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sale_extra")); err != nil {
		t.Fatalf("addon was touched: %v", err)
	}
}
