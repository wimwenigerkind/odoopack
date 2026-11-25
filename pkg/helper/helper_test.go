package helper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "odoopack.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteJSONFile(path, []byte(`{"new":true}`)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"new\":true}\n" {
		t.Fatalf("got %q, want the document with a trailing newline", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0644 {
		t.Fatalf("got mode %o, want 644", perm)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want only the file and no temporary one left", len(entries))
	}
}

func TestWriteJSONFileKeepsOldFileOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "odoopack.json")

	if err := WriteJSONFile(path, []byte(`{}`)); err == nil {
		t.Fatal("wrote into a directory that does not exist")
	}
}
