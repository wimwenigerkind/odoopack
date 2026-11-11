package installer

import (
	"fmt"
	"strings"

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

func FormatAddonDir(name string) string {
	return strings.NewReplacer("/", "_", "-", "_").Replace(name)
}
