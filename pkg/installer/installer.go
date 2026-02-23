package installer

import (
	"fmt"

	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type Installer interface {
	Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error
}

func New(sourceType index.Type) (Installer, error) {
	switch sourceType {
	case index.TypeGit:
		return NewGitInstaller(), nil
	default:
		return nil, fmt.Errorf("installer type is not supported")
	}
}
