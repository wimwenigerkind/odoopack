/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var requireCmd = &cobra.Command{
	Use:     "require <addon>[@<version>]",
	Short:   "Add an addon dependency",
	Aliases: []string{"add", "req"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addon := args[0]
		addonName, versionArg, _ := strings.Cut(addon, "@")

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		checkAddonsPath(m)
		oldLock, err := lockfile.LoadOrNew()
		if err != nil {
			fatal(err)
		}

		constraint := versionArg
		if constraint == "" {
			constraint = m.OdooSeries
		}
		if err := manifest.ValidConstraint(constraint, m.OdooSeries); err != nil {
			fatal(fmt.Errorf("%s: %w", addon, err))
		}

		lookup, err := index.LookupLocked(m.Indexes, oldLock.Packages[addonName].IndexURL, addonName, constraint, m.OdooSeries)
		if err != nil {
			fatal(err)
		}

		m.AddRequirement(lookup.Name, constraint)
		lockFile, err := lockfile.RecomputeHash(m.Require, m.Indexes, m.OdooSeries, oldLock.Packages)
		if err != nil {
			fatal(err)
		}
		if err := addonsdir.Claim(m.AddonsDir, lockedDirs(oldLock.Packages, lockFile.Packages)); err != nil {
			fatal(err)
		}
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}
		if err := lockfile.Save(lockFile); err != nil {
			fatal(err)
		}

		installed, removed, err := applyLock(m, oldLock, lockFile)
		if err != nil {
			fatal(err)
		}

		ui.Success("added %s@%s (%d package(s) installed, %d removed)", lookup.Name, lockFile.Packages[lookup.Name].Version, installed, removed)
	},
}

func init() {
	addNoCacheFlag(requireCmd)
	rootCmd.AddCommand(requireCmd)
}
