package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func fillAddon(dst string) error {
	if err := os.MkdirAll(filepath.Join(dst, "models"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dst, "__manifest__.py"), []byte("{}"), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, "models", "a.py"), []byte("a"), 0644)
}

func TestEnsureFillsOnce(t *testing.T) {
	st := New(t.TempDir())
	calls := 0
	fill := func(dst string) error {
		calls++
		return fillAddon(dst)
	}

	first, err := st.Ensure(digest, fill)
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.Ensure(digest, fill)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || calls != 1 {
		t.Fatalf("got entries %q and %q after %d fills, want one entry from one fill", first, second, calls)
	}

	info, err := os.Stat(filepath.Join(first, "__manifest__.py"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0222 != 0 {
		t.Fatalf("store file mode %v is writable", info.Mode().Perm())
	}
}

func TestEnsureKeepsNothingOnFailure(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	_, err := st.Ensure(digest, func(dst string) error {
		if err := fillAddon(dst); err != nil {
			return err
		}
		return os.ErrInvalid
	})
	if err == nil {
		t.Fatal("expected the fill error")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("store keeps %d entries after a failed fill", len(entries))
	}
}

func TestEnsureRejectsInvalidDigest(t *testing.T) {
	st := New(t.TempDir())
	for _, d := range []string{"", "../../etc", strings.ToUpper(digest), digest[:63]} {
		if _, err := st.Ensure(d, fillAddon); err == nil {
			t.Errorf("digest %q accepted", d)
		}
	}
}

func TestLinkSharesFiles(t *testing.T) {
	st := New(t.TempDir())
	entry, err := st.Ensure(digest, fillAddon)
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "addon")
	if err := Link(entry, dest); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"__manifest__.py", filepath.Join("models", "a.py")} {
		src, err := os.Stat(filepath.Join(entry, rel))
		if err != nil {
			t.Fatal(err)
		}
		dst, err := os.Stat(filepath.Join(dest, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(src, dst) {
			t.Errorf("%s is not hard linked to the store", rel)
		}
	}

	if err := os.RemoveAll(dest); err != nil {
		t.Fatalf("linked addon cannot be removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(entry, "__manifest__.py")); err != nil {
		t.Fatalf("removing the addon removed the store entry: %v", err)
	}
}

func TestCopyFileIsWritable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0444); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst")
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0200 == 0 {
		t.Fatalf("copied file mode %v is not writable", info.Mode().Perm())
	}
}

func TestPruneKeepsLinkedEntries(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	used, err := st.Ensure(digest, fillAddon)
	if err != nil {
		t.Fatal(err)
	}
	unused, err := st.Ensure(strings.Repeat("f", 64), fillAddon)
	if err != nil {
		t.Fatal(err)
	}
	if err := Link(used, filepath.Join(t.TempDir(), "addon")); err != nil {
		t.Fatal(err)
	}

	res, err := st.Prune(false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Entries != 1 || res.Bytes == 0 {
		t.Fatalf("got %+v, want one entry with its bytes", res)
	}
	if _, err := os.Stat(used); err != nil {
		t.Errorf("linked entry removed: %v", err)
	}
	if _, err := os.Stat(unused); !os.IsNotExist(err) {
		t.Errorf("unlinked entry kept: %v", err)
	}
}

func TestPruneAllRemovesLinkedEntries(t *testing.T) {
	st := New(t.TempDir())
	entry, err := st.Ensure(digest, fillAddon)
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "addon")
	if err := Link(entry, project); err != nil {
		t.Fatal(err)
	}

	res, err := st.Prune(true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Entries != 1 {
		t.Fatalf("got %+v, want one entry", res)
	}
	if _, err := os.Stat(filepath.Join(project, "__manifest__.py")); err != nil {
		t.Errorf("project lost its files: %v", err)
	}
}

func TestPruneStagingAndForeignEntries(t *testing.T) {
	dir := t.TempDir()
	st := New(dir)
	fresh := filepath.Join(dir, ".tmp-fresh")
	stale := filepath.Join(dir, ".tmp-stale")
	foreign := filepath.Join(dir, "notes")
	for _, p := range []string{fresh, stale, foreign} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * staleStagingAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	res, err := st.Prune(false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Entries != 0 {
		t.Fatalf("staging directories counted as entries: %+v", res)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale staging directory kept: %v", err)
	}
	for _, p := range []string{fresh, foreign} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s removed: %v", filepath.Base(p), err)
		}
	}
}

func TestPruneMissingStore(t *testing.T) {
	res, err := New(filepath.Join(t.TempDir(), "missing")).Prune(false)
	if err != nil || res != (PruneResult{}) {
		t.Fatalf("got %+v, %v", res, err)
	}
}
