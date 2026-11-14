package installer

import (
	"encoding/pem"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

// gitRepo is a bare repository served over https on the loopback interface
// with git http-backend, since the installer refuses every other transport.
type gitRepo struct {
	t    *testing.T
	work string
	url  string
}

func newGitRepo(t *testing.T) *gitRepo {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "acme")
	t.Setenv("GIT_AUTHOR_EMAIL", "dev@acme.example")
	t.Setenv("GIT_COMMITTER_NAME", "acme")
	t.Setenv("GIT_COMMITTER_EMAIL", "dev@acme.example")

	root := t.TempDir()
	r := &gitRepo{t: t, work: filepath.Join(root, "work")}
	r.git(root, "init", "--quiet", "--bare", "acme.git")
	r.git(root, "init", "--quiet", "--initial-branch", "main", "work")
	r.git(r.work, "remote", "add", "origin", filepath.Join(root, "acme.git"))

	srv := httptest.NewTLSServer(&cgi.Handler{
		Path: gitPath,
		Args: []string{"http-backend"},
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull},
	})
	t.Cleanup(srv.Close)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caFile, ca, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSL_CAINFO", caFile)
	r.url = srv.URL + "/acme.git"
	return r
}

func (r *gitRepo) git(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit records a manifest with body and pushes the branch.
func (r *gitRepo) commit(body string) string {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.work, "__manifest__.py"), []byte(body), 0644); err != nil {
		r.t.Fatal(err)
	}
	r.git(r.work, "add", ".")
	r.git(r.work, "commit", "--quiet", "-m", body)
	r.git(r.work, "push", "--quiet", "origin", "HEAD")
	return r.git(r.work, "rev-parse", "HEAD")
}

func (r *gitRepo) tag(name, commit string) {
	r.t.Helper()
	r.git(r.work, "tag", "--force", name, commit)
	r.git(r.work, "push", "--quiet", "--force", "origin", "refs/tags/"+name)
}

func (r *gitRepo) locked(commit string) lockfile.LockedPackage {
	return lockfile.LockedPackage{
		Version: "19.0.1.0.0",
		Dist:    lockfile.Dist{Type: "git", URL: r.url, Reference: commit},
	}
}

func installedManifest(t *testing.T, target string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(target, "acme_health_check", "__manifest__.py"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "acme_health_check", ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git is left in the addon: %v", err)
	}
	return string(got)
}

// useProtocolV0 makes the client speak protocol v0, where http-backend only
// serves commits a ref points at, so a fetch by commit id fails and the
// installer has to fall back to fetching the tag.
func useProtocolV0(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.version")
	t.Setenv("GIT_CONFIG_VALUE_0", "0")
}

func TestGitInstallChecksOutLockedCommitAfterTagMoved(t *testing.T) {
	for _, tt := range []struct {
		name     string
		protocol func(*testing.T)
	}{
		{"fetch by commit", func(*testing.T) {}},
		{"fallback to the tag", useProtocolV0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newGitRepo(t)
			locked := r.commit("locked")
			r.tag("19.0.1.0.0", locked)
			r.tag("19.0.1.0.0", r.commit("moved"))
			tt.protocol(t)

			target := t.TempDir()
			if err := NewGitInstaller().Install(target, "acme/health-check", r.locked(locked)); err != nil {
				t.Fatal(err)
			}
			if got := installedManifest(t, target); got != "locked" {
				t.Fatalf("installed %q, want the locked commit", got)
			}
		})
	}
}

func TestGitInstallFailsWhenLockedCommitIsGone(t *testing.T) {
	r := newGitRepo(t)
	locked := r.commit("locked")
	r.tag("19.0.1.0.0", locked)
	// Rewrite the branch and the tag so that nothing reaches the locked
	// commit any more.
	r.git(r.work, "checkout", "--quiet", "--orphan", "rewritten")
	other := r.commit("rewritten")
	r.git(r.work, "push", "--quiet", "--force", "origin", "rewritten:main")
	r.tag("19.0.1.0.0", other)
	useProtocolV0(t)

	target := t.TempDir()
	err := NewGitInstaller().Install(target, "acme/health-check", r.locked(locked))
	if err == nil || !strings.Contains(err.Error(), "is not in") {
		t.Fatalf("got error %v, want one saying the commit is missing", err)
	}
	if _, err := os.Stat(filepath.Join(target, "acme_health_check")); !os.IsNotExist(err) {
		t.Fatalf("addon was installed: %v", err)
	}
}

func TestGitInstallRejectsHeadMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a shell script")
	}
	r := newGitRepo(t)
	locked := r.commit("locked")

	// A post-checkout hook moves HEAD to another commit, standing in for
	// anything that leaves a different commit checked out.
	hooks := t.TempDir()
	hook := "#!/bin/sh\ngit update-ref --no-deref HEAD \"$(git commit-tree -m other HEAD^{tree})\"\n"
	if err := os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte(hook), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", hooks)

	err := NewGitInstaller().Install(t.TempDir(), "acme/health-check", r.locked(locked))
	if err == nil || !strings.Contains(err.Error(), "but the lockfile pins "+locked) {
		t.Fatalf("got error %v, want a HEAD mismatch", err)
	}
}

func TestGitInstallRequiresReference(t *testing.T) {
	for _, reference := range []string{"", "19.0.1.0.0", "abc123", strings.Repeat("A", 40)} {
		pkg := lockfile.LockedPackage{Version: "19.0.1.0.0", Dist: lockfile.Dist{Type: "git", URL: "https://example.com/acme.git", Reference: reference}}
		err := NewGitInstaller().Install(t.TempDir(), "acme/health-check", pkg)
		if err == nil || !strings.Contains(err.Error(), "odoopack update acme/health-check") {
			t.Errorf("reference %q: got error %v, want one asking to lock again", reference, err)
		}
	}
}

func TestGitInstallRefusesURL(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, url := range []string{
		"file:///srv/git/acme.git",
		"http://example.com/acme.git",
		"git://example.com/acme.git",
		"ext::sh -c touch% /tmp/pwned",
		"--upload-pack=touch /tmp/pwned",
		"-oProxyCommand=touch /tmp/pwned:acme.git",
		"git@-oProxyCommand=x:acme.git",
		"ssh://-oProxyCommand=x/acme.git",
		"/srv/git/acme.git",
		"./acme.git",
		"C:/git/acme.git",
		"",
	} {
		pkg := lockfile.LockedPackage{Dist: lockfile.Dist{Type: "git", URL: url, Reference: commit}}
		target := filepath.Join(t.TempDir(), "addons")
		if err := NewGitInstaller().Install(target, "acme/health-check", pkg); err == nil {
			t.Errorf("url %q was accepted", url)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("url %q: target was created: %v", url, err)
		}
	}
}

func TestCheckGitURLAcceptsHTTPSAndSSH(t *testing.T) {
	for _, url := range []string{
		"https://git.acme.example/acme/health-check.git",
		"ssh://git@git.acme.example:2222/acme/health-check.git",
		"git@git.acme.example:acme/health-check.git",
		"git.acme.example:acme/health-check.git",
	} {
		if err := checkGitURL(url); err != nil {
			t.Errorf("url %q: %v", url, err)
		}
	}
}

// The protocol rules hold in git itself, not only in checkGitURL.
func TestGitCommandBlocksLocalRepository(t *testing.T) {
	r := newGitRepo(t)
	locked := r.commit("locked")
	bare := filepath.Join(filepath.Dir(r.work), "acme.git")

	for _, url := range []string{"file://" + filepath.ToSlash(bare), bare} {
		err := checkoutCommit(t.TempDir(), url, "", locked)
		if err == nil {
			t.Errorf("url %q was fetched", url)
		}
	}
}
