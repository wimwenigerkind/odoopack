package installer

import (
	"fmt"
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
	tmpDir, err := os.MkdirTemp("", "odoopack-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	args := []string{"clone", "--depth", "1"}
	if pkg.Version != "" && pkg.Version != "latest" {
		args = append(args, "--branch", pkg.Version)
	}
	args = append(args, pkg.Repository, tmpDir)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s\n%s", err, string(output))
	}

	err = os.RemoveAll(filepath.Join(tmpDir, ".git"))
	if err != nil {
		return err
	}

	addonDir := strings.ReplaceAll(addonName, "/", "_")

	dest := filepath.Join(targetDir, addonDir)

	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return err
	}

	err = os.RemoveAll(dest)
	if err != nil {
		return err
	}

	err = os.Rename(tmpDir, dest)
	if err != nil {
		return err
	}
	return nil
}
