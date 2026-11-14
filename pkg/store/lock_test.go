package store

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const pruneDirEnv = "ODOOPACK_TEST_PRUNE_DIR"

// installConcurrently installs digests into projects from many goroutines,
// each through its own Store as the installer does, and removes most projects
// again so prune has unused entries to take. It returns the entries that stay
// linked.
func installConcurrently(t *testing.T, dir string) (kept, unused []string) {
	t.Helper()
	digests := make([]string, 8)
	for i := range digests {
		digests[i] = strings.Repeat(fmt.Sprintf("%x", i), 64)
	}
	const workers, rounds = 16, 25
	projects := t.TempDir()

	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for r := range rounds {
				st := New(dir)
				entry, err := st.Ensure(digests[(w+r)%len(digests)], fillAddon)
				if err != nil {
					t.Errorf("ensure: %v", err)
					return
				}
				dest := filepath.Join(projects, fmt.Sprint(w, "-", r))
				if err := st.Link(entry, dest); err != nil {
					t.Errorf("link: %v", err)
					return
				}
				if r == rounds-1 && w < len(digests)/2 {
					continue
				}
				if err := os.RemoveAll(dest); err != nil {
					t.Errorf("remove project: %v", err)
					return
				}
			}
		})
	}
	wg.Wait()

	for i, d := range digests {
		// Worker w keeps its last round, which used digest (w+rounds-1).
		if (i-(rounds-1)%len(digests)+len(digests))%len(digests) < len(digests)/2 {
			kept = append(kept, filepath.Join(dir, d))
		} else {
			unused = append(unused, filepath.Join(dir, d))
		}
	}
	return kept, unused
}

func assertPruned(t *testing.T, dir string, kept, unused []string) {
	t.Helper()
	if _, err := New(dir).Prune(false); err != nil {
		t.Fatal(err)
	}
	for _, entry := range kept {
		if _, err := os.Stat(entry); err != nil {
			t.Errorf("linked entry removed: %v", err)
		}
	}
	for _, entry := range unused {
		if _, err := os.Stat(entry); !os.IsNotExist(err) {
			t.Errorf("unlinked entry %s kept: %v", filepath.Base(entry), err)
		}
	}
}

func TestPruneDuringInstalls(t *testing.T) {
	dir := t.TempDir()
	stop := make(chan struct{})
	pruned := make(chan error)
	go func() {
		for {
			select {
			case <-stop:
				close(pruned)
				return
			default:
			}
			if _, err := New(dir).Prune(false); err != nil {
				pruned <- err
			}
		}
	}()

	kept, unused := installConcurrently(t, dir)
	close(stop)
	for err := range pruned {
		t.Errorf("prune: %v", err)
	}
	assertPruned(t, dir, kept, unused)
}

func TestPruneInOtherProcessDuringInstalls(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperPruneLoop$")
	cmd.Env = append(os.Environ(), pruneDirEnv+"="+dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	kept, unused := installConcurrently(t, dir)
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("prune process: %v\n%s", err, out.String())
	}
	assertPruned(t, dir, kept, unused)
}

// TestHelperPruneLoop is the other process of
// TestPruneInOtherProcessDuringInstalls: it prunes until its stdin closes.
func TestHelperPruneLoop(t *testing.T) {
	dir := os.Getenv(pruneDirEnv)
	if dir == "" {
		t.Skip("runs only as a child process")
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(done)
	}()
	for {
		select {
		case <-done:
			return
		default:
		}
		if _, err := New(dir).Prune(false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLockFileMode(t *testing.T) {
	dir := t.TempDir()
	ensureLinked(t, New(dir), digest, fillAddon)
	info, err := os.Stat(filepath.Join(dir, lockName))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("lock file mode %v is not a regular file", info.Mode())
	}
	// Windows reports every writable file as 0666.
	if runtime.GOOS == "windows" {
		return
	}
	if perm := info.Mode().Perm(); perm&0600 != 0600 || perm&^0644 != 0 {
		t.Fatalf("lock file mode %v, want 0644 or narrower by umask", perm)
	}
}
