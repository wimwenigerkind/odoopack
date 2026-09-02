/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
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

		if len(m.Require) == 0 {
			ui.Info("no addons required")
			return
		}

		lock := lockfile.LoadOrNew()

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

		if err := installAll(m, lock); err != nil {
			ui.Error("install failed: %v", err)
			os.Exit(1)
		}
		ui.Success("installed %d package(s)", len(lock.Packages))
	},
}

func installAll(m *manifest.Manifest, lock lockfile.LockFile) error {
	if err := os.RemoveAll(m.AddonsPath); err != nil {
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

func installOne(m *manifest.Manifest, name string, pkg lockfile.LockedPackage) error {
	inst, err := installer.New(pkg.Dist.Type)
	if err != nil {
		return err
	}
	return inst.Install(m.AddonsPath, name, pkg)
}

func init() {
	rootCmd.AddCommand(installCmd)
}
