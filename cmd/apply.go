/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"

	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

// applyLock changes addons_path from the packages of prev to those of next,
// touching only what differs. It returns how many packages it installed and
// removed. The caller claims addons_path before it saves the manifest or the
// lockfile, so that nothing is saved for a directory odoopack does not own.
func applyLock(m *manifest.Manifest, prev, next lockfile.LockFile) (installed, removed int, err error) {
	install, remove := lockfile.Plan(prev.Packages, next.Packages)
	owned := lockedDirs(prev.Packages, next.Packages)

	// Removing first frees the directory of a dropped package for a new one
	// with the same module name.
	for _, pkg := range remove {
		if err := addonsdir.RemoveAddon(m.AddonsPath, name.ModuleName(pkg), owned); err != nil {
			return 0, 0, err
		}
	}
	if len(install) == 0 {
		return 0, len(remove), nil
	}

	tasks := make([]ui.Task, len(install))
	for i, pkg := range install {
		tasks[i] = ui.Task{Label: pkg + "@" + next.Packages[pkg].Version}
	}
	err = ui.RunTasks(fmt.Sprintf("installing %d package(s)", len(install)), tasks, func(i int) error {
		if err := installOne(m, install[i], next.Packages[install[i]]); err != nil {
			return fmt.Errorf("%s: %w", install[i], err)
		}
		return nil
	})
	if err != nil {
		return 0, len(remove), err
	}
	return len(install), len(remove), nil
}
