/*
Copyright © 2026 Wim
*/
package cmd

import (
	"fmt"
	"maps"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var removeCmd = &cobra.Command{
	Use:   "remove [addon]",
	Short: "Remove a addon from requirements",
	Args:  cobra.RangeArgs(1, 1),
	Run: func(cmd *cobra.Command, args []string) {
		addon := args[0]
		addonParts := strings.Split(addon, "@")
		addonName := addonParts[0]

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		checkAddonsPath(m)

		if len(m.Require) == 0 {
			ui.Info("no addons required")
			return
		}

		version, ok := m.Require[addonName]
		if !ok {
			fatal(fmt.Errorf("addon is not installed"))
		}

		lock, err := lockfile.LoadOrNew()
		if err != nil {
			fatal(err)
		}
		remaining := maps.Clone(m.Require)
		delete(remaining, addonName)
		next, err := lockfile.Relock(remaining, m.Indexes, m.Odoo, lock.Packages)
		if err != nil {
			fatal(err)
		}
		if _, needed := next.Packages[addonName]; needed {
			fatal(fmt.Errorf("cannot remove %s: %s depend(s) on it", addonName, strings.Join(lockfile.Dependents(next.Packages, addonName), ", ")))
		}

		if err := addonsdir.Claim(m.AddonsPath, lockedDirs(lock.Packages, next.Packages)); err != nil {
			fatal(err)
		}
		m.RemoveRequirement(addonName)
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}
		if err := lockfile.Save(next); err != nil {
			fatal(err)
		}

		installed, removed, err := applyLock(m, lock, next)
		if err != nil {
			fatal(err)
		}

		ui.Success("removed %s@%s (%d package(s) removed, %d installed)", addonName, version, removed, installed)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
