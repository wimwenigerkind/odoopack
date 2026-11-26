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

// addonDir checks the module again, since the lockfile may be edited by hand.
func addonDir(targetDir string, pkg lockfile.LockedPackage) (string, error) {
	if err := name.ValidModule(pkg.Module); err != nil {
		return "", err
	}
	return filepath.Join(targetDir, pkg.Module), nil
}

func checkIntegrity(addonName string, pkg lockfile.LockedPackage) error {
	if err := pkg.Dist.CheckIntegrity(); err != nil {
		return fmt.Errorf("%w; run \"odoopack update %s\" to lock it again", err, addonName)
	}
	return nil
}
