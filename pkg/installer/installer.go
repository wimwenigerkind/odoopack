package installer

import (
	"fmt"
	"strings"

	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
)

type Installer interface {
	Install(targetDir string, addonName string, pkg lockfile.LockedPackage) error
}

func New(sourceType string) (Installer, error) {
	switch sourceType {
	case "git":
		return NewGitInstaller(), nil
	case "zip":
		return NewZipInstaller(), nil
	default:
		return nil, fmt.Errorf("installer type %q is not supported", sourceType)
	}
}

func FormatAddonDir(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}
