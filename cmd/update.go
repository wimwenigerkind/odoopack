/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

var updateCmd = &cobra.Command{
	Use:     "update [addon]",
	Short:   "Re-resolve requirements against the registry and reinstall",
	Aliases: []string{"up"},
	Args:    cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		if len(m.Require) == 0 {
			fmt.Println("nothing to update")
			return
		}

		if len(args) == 1 {
			updateOne(m, args[0])
			return
		}
		updateAll(m)
	},
}

func updateAll(m *manifest.Manifest) {
	lock, err := lockfile.RecomputeHash(m.Require, m.Indexes)
	if err != nil {
		fatal(err)
	}
	if err := lockfile.Save(lock); err != nil {
		fatal(err)
	}
	if err := installAll(m, lock); err != nil {
		fatal(err)
	}
	fmt.Printf("updated %d addon(s)\n", len(lock.Packages))
}

func updateOne(m *manifest.Manifest, name string) {
	version, ok := m.Require[name]
	if !ok {
		fatal(fmt.Errorf("%s is not a required addon", name))
	}

	lookup, err := index.Lookup(m.Indexes, name, version)
	if err != nil {
		fatal(err)
	}

	lock := lockfile.LoadOrNew()
	lock.Packages[lookup.Name] = lockfile.LockedPackage{
		Version:    lookup.Version,
		Type:       lookup.Type,
		Repository: lookup.Repository,
	}
	lock.ContentHash, err = lockfile.ComputeHash(m.Require, m.Indexes)
	if err != nil {
		fatal(err)
	}
	if err := lockfile.Save(lock); err != nil {
		fatal(err)
	}

	if err := installOne(m, lookup.Name, lock.Packages[lookup.Name]); err != nil {
		fatal(err)
	}
	fmt.Printf("updated %s@%s\n", lookup.Name, lookup.Version)
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
