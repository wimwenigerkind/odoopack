package installer

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type GitInstaller struct{}

func NewGitInstaller() *GitInstaller {
	return &GitInstaller{}
}

func (i *GitInstaller) Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error {
	dest, err := addonDir(targetDir, addonName)
	if err != nil {
		return err
	}
	if err := checkIntegrity(addonName, pkg); err != nil {
		return err
	}
	if err := checkGitURL(pkg.Dist.URL); err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(targetDir, ".odoopack-tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := checkoutCommit(tmpDir, pkg.Dist.URL, pkg.Version, pkg.Dist.Reference); err != nil {
		return err
	}

	if err := os.RemoveAll(filepath.Join(tmpDir, ".git")); err != nil {
		return err
	}

	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(tmpDir, dest)
}

// checkoutCommit checks out commit from repoURL into the empty directory dir.
// The tag or branch named version may have moved since the package was locked,
// so it only serves as a fallback for hosts that refuse to fetch a commit by
// its id.
func checkoutCommit(dir, repoURL, version, commit string) error {
	initArgs := []string{"init", "--quiet"}
	if len(commit) == 64 {
		initArgs = append(initArgs, "--object-format=sha256")
	}
	if err := runGit(dir, initArgs...); err != nil {
		return err
	}

	if fetchErr := runGit(dir, "fetch", "--quiet", "--depth", "1", "--", repoURL, commit); fetchErr != nil {
		// Without a refspec git fetches the remote HEAD.
		args := []string{"fetch", "--quiet", "--", repoURL}
		if version != "" && version != "latest" {
			args = append(args, version)
		}
		if err := runGit(dir, args...); err != nil {
			return fmt.Errorf("fetch commit %s: %w\nfallback: %w", commit, fetchErr, err)
		}
	}
	if err := runGit(dir, "checkout", "--quiet", "--detach", commit); err != nil {
		return fmt.Errorf("commit %s is not in %s: %w", commit, repoURL, err)
	}

	out, err := gitCommand(dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	if head := strings.TrimSpace(string(out)); head != commit {
		return fmt.Errorf("checked out %s, but the lockfile pins %s", head, commit)
	}
	return nil
}

func runGit(dir string, args ...string) error {
	output, err := gitCommand(dir, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", args[0], err, output)
	}
	return nil
}

// gitCommand runs git in dir without letting a lockfile reach anything but a
// remote https or ssh repository: no local paths, no remote helpers, and no
// prompt that would hang a non-interactive install.
func gitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-c", "protocol.file.allow=never"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=https:ssh")
	return cmd
}

// checkGitURL accepts https and ssh URLs and the scp-like ssh form
// user@host:path. GIT_ALLOW_PROTOCOL enforces the same in git; checking here
// as well gives a clear error and refuses what git would read as an option.
func checkGitURL(raw string) error {
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("git url %q must not start with -", raw)
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("git url %q: %w", raw, err)
		}
		if u.Scheme != "https" && u.Scheme != "ssh" {
			return fmt.Errorf("git url %q must use https or ssh", raw)
		}
		if u.Hostname() == "" || strings.HasPrefix(u.Hostname(), "-") {
			return fmt.Errorf("git url %q has an invalid host", raw)
		}
		return nil
	}

	// git reads host:path as ssh when the colon comes before any slash,
	// host::address as a remote helper, and a single letter before the
	// colon as a Windows drive.
	userHost, path, ok := strings.Cut(raw, ":")
	host := userHost
	if _, h, found := strings.Cut(userHost, "@"); found {
		host = h
	}
	if !ok || strings.Contains(userHost, "/") || len(host) < 2 || strings.HasPrefix(host, "-") || path == "" || strings.HasPrefix(path, ":") {
		return fmt.Errorf("git url %q must be an https or ssh url", raw)
	}
	return nil
}
