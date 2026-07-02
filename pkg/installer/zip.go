package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wimwenigerkind/odoopack/pkg/auth"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type ZipInstaller struct{}

func NewZipInstaller() *ZipInstaller {
	return &ZipInstaller{}
}

func (i *ZipInstaller) Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	tmp, err := downloadToTmp(pkg.Dist.URL)
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := verifyShasum(tmpPath, pkg.Dist.Shasum); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp(targetDir, ".odoopack-unzip-*")
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
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(filepath.Join(tmpDir, entries[0].Name()), dest)
}

func downloadToTmp(url string) (*os.File, error) {
	tmp, err := os.CreateTemp("", "odoopack-*.zip")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := auth.TokenForURL(url); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		os.Remove(tmp.Name())
		return nil, fmt.Errorf("download %s: bad status %s", url, response.Status)
	}

	if _, err = io.Copy(tmp, response.Body); err != nil {
		return nil, err
	}

	return tmp, nil
}

func verifyShasum(path, expected string) error {
	if expected == "" {
		return nil
	}
	algo, want, ok := strings.Cut(expected, ":")
	if !ok || algo != "sha256" {
		return fmt.Errorf("unsupported shasum format %q (want sha256:<hex>)", expected)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("shasum mismatch: got sha256:%s, expected %s — refusing to install a corrupt or tampered zipball", got, expected)
	}
	return nil
}

func unzip(zipPath, destDir string) error {
	out, err := exec.Command("unzip", "-o", zipPath, "-d", destDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unzip failed: %s: %w", out, err)
	}
	return nil
}
