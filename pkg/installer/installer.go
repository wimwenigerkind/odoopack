package installer

import (
	"fmt"
	"path/filepath"

	"github.com/wimwenigerkind/odoopack-spec/name"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/store"
)

type Installer interface {
	Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error
}

func New(sourceType string) (Installer, error) {
	switch sourceType {
	case "git":
		return NewGitInstaller(), nil
	case "zip":
		st, err := store.Default()
		if err != nil {
			return nil, err
		}
		return NewZipInstaller(st), nil
	default:
		return nil, fmt.Errorf("installer type %q is not supported", sourceType)
	}
}

// addonDir returns the directory addonName installs to in targetDir.
func addonDir(targetDir, addonName string) (string, error) {
	if err := name.ValidPackage(addonName); err != nil {
		return "", err
	}
	return filepath.Join(targetDir, name.ModuleName(addonName)), nil
}

// checkIntegrity refuses a package the installer could not verify, such as a
// zip locked by an odoopack that did not require a digest yet.
func checkIntegrity(addonName string, pkg lockfile.LockedPackage) error {
	if err := pkg.Dist.CheckIntegrity(); err != nil {
		return fmt.Errorf("%w; run \"odoopack update %s\" to lock it again", err, addonName)
	}
	return nil
}
