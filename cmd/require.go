/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

var requireCmd = &cobra.Command{
	Use:     "require [addon]@[version]",
	Short:   "Add an addon dependency",
	Aliases: []string{"add", "req"},
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		addon := args[0]
		addonParts := strings.Split(addon, "@")
		if len(addonParts) != 2 || addonParts[1] == "" {
			fatal(fmt.Errorf("version required: use %s@<version> (e.g. 18.0.1.0.0 or dev-18.0)", addonParts[0]))
		}
		addonName := addonParts[0]
		constraint := addonParts[1]

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		lookup, err := index.Lookup(m.Indexes, addonName, constraint, m.Odoo)
		if err != nil {
			fatal(err)
		}

		m.AddRequirement(lookup.Name, constraint)
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}

		oldLock := lockfile.LoadOrNew()

		lockFile, err := lockfile.RecomputeHash(m.Require, m.Indexes, m.Odoo)
		if err != nil {
			fatal(err)
		}
		if err := lockfile.Save(lockFile); err != nil {
			fatal(err)
		}

		added := 0
		for name, pkg := range lockFile.Packages {
			if _, existed := oldLock.Packages[name]; existed {
				continue
			}
			if err := installOne(m, name, pkg); err != nil {
				fatal(err)
			}
			added++
		}

		fmt.Printf("Added %s@%s (%d package(s) installed)\n", lookup.Name, lookup.Version, added)
	},
}

func init() {
	rootCmd.AddCommand(requireCmd)
}
