/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var installCmd = &cobra.Command{
	Use:     "install",
	Short:   "Install all required addons",
	Aliases: []string{"i"},
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		checkAddonsPath(m)

		if len(m.Require) == 0 {
			ui.Info("no addons required")
			return
		}

		lock, err := lockfile.LoadOrNew()
		if err != nil {
			fatal(err)
		}
		previous := lock

		isStale, err := lockfile.IsStale(m.Require, m.Indexes, lock.Packages, lock.ContentHash)
		if err != nil {
			fatal(err)
		}

		if isStale {
			ui.Info("lockfile is stale, resolving")
			lock, err = lockfile.RecomputeHash(m.Require, m.Indexes, m.Odoo)
			if err != nil {
				fatal(err)
			}
			if err := lockfile.Save(lock); err != nil {
				fatal(err)
			}
		}

		if err := installAll(m, lock, previous); err != nil {
			ui.Error("install failed: %v", err)
			os.Exit(1)
		}
		ui.Success("installed %d package(s)", len(lock.Packages))
	},
}

// installAll replaces addons_path with the packages in lock. previous is the
// lockfile before this run resolved again: an install from an older odoopack
// without the marker holds the addons of that one, and must still be adopted.
func installAll(m *manifest.Manifest, lock, previous lockfile.LockFile) error {
	if err := addonsdir.Clear(m.AddonsPath, lockedDirs(lock.Packages, previous.Packages)); err != nil {
		return err
	}

	names := make([]string, 0, len(lock.Packages))
	for name := range lock.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	tasks := make([]ui.Task, len(names))
	for i, name := range names {
		tasks[i] = ui.Task{Label: name + "@" + lock.Packages[name].Version}
	}

	return ui.RunTasks(fmt.Sprintf("installing %d package(s)", len(names)), tasks, func(i int) error {
		name := names[i]
		if err := installOne(m, name, lock.Packages[name]); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	})
}

// checkAddonsPath runs before a command writes to addons_path, so that nothing
// is created or deleted for an invalid one.
func checkAddonsPath(m *manifest.Manifest) {
	wd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	if err := addonsdir.Check(wd, m.AddonsPath); err != nil {
		fatal(err)
	}
}

func lockedDirs(locks ...map[string]lockfile.LockedPackage) map[string]bool {
	dirs := make(map[string]bool)
	for _, packages := range locks {
		for addon := range packages {
			dirs[name.ModuleName(addon)] = true
		}
	}
	return dirs
}

func installOne(m *manifest.Manifest, name string, pkg lockfile.LockedPackage) error {
	inst, err := installer.New(pkg.Dist.Type)
	if err != nil {
		return err
	}
	return inst.Install(m.AddonsPath, name, pkg)
}

func init() {
	addNoCacheFlag(installCmd)
	rootCmd.AddCommand(installCmd)
}
