package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type GitInstaller struct{}

func NewGitInstaller() *GitInstaller {
	return &GitInstaller{}
}

func (i *GitInstaller) Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(targetDir, ".odoopack-tmp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	args := []string{"clone", "--depth", "1"}
	if pkg.Version != "" && pkg.Version != "latest" {
		args = append(args, "--branch", pkg.Version)
	}
	args = append(args, pkg.Dist.URL, tmpDir)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s\n%s", err, string(output))
	}

	if err := os.RemoveAll(filepath.Join(tmpDir, ".git")); err != nil {
		return err
	}

	dest := filepath.Join(targetDir, FormatAddonDir(addonName))
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(tmpDir, dest)
}
