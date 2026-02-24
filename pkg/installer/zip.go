package installer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type ZipInstaller struct{}

func NewZipInstaller() *ZipInstaller {
	return &ZipInstaller{}
}

func (i *ZipInstaller) Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error {
	tmp, err := downloadToTmp(pkg.Repository)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	tmpDir, err := os.MkdirTemp("", "odoopack-unzip-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := unzip(tmpPath, tmpDir); err != nil {
		return err
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return fmt.Errorf("expected zip to contain a single root directory")
	}

	dest := filepath.Join(targetDir, FormatAddonDir(addonName))

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	err = os.RemoveAll(dest)
	if err != nil {
		return err
	}

	return os.Rename(filepath.Join(tmpDir, entries[0].Name()), dest)
}

func downloadToTmp(url string) (*os.File, error) {
	tmp, err := os.CreateTemp("", "odoopack-*.zip")
	if err != nil {
		return nil, err
	}

	client := &http.Client{}

	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		os.Remove(tmp.Name())
		return nil, fmt.Errorf("bad status %s", response.Status)
	}

	if _, err = io.Copy(tmp, response.Body); err != nil {
		return nil, err
	}

	return tmp, nil
}

func unzip(zipPath, destDir string) error {
	out, err := exec.Command("unzip", "-o", zipPath, "-d", destDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unzip failed: %s: %w", out, err)
	}
	return nil
}
