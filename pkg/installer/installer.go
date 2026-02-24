package installer

import (
	"fmt"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type Installer interface {
	Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error
}

func New(sourceType string) (Installer, error) {
	switch sourceType {
	case "git":
		return NewGitInstaller(), nil
	default:
		return nil, fmt.Errorf("installer type %q is not supported", sourceType)
	}
}
