/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
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
		addonName := addonParts[0]
		version := "latest"
		if len(addonParts) > 1 {
			version = addonParts[1]
		}

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		indexProvider := index.StaticProvider{
			Endpoint: m.Indexes["default"].Url,
		}

		lookup, err := indexProvider.Lookup(addonName, version)
		if err != nil {
			fatal(err)
		}

		if err := m.AddRequirement(lookup.Name, lookup.Version); err != nil {
			fatal(err)
		}

		lockFile := lockfile.LoadOrNew()

		lockFile.Packages[lookup.Name] = lockfile.LockedPackage{
			Version:    lookup.Version,
			Type:       lookup.Type,
			Repository: lookup.Repository,
		}

		lockFile.ContentHash, err = lockfile.ComputeHash(m.Require)
		if err != nil {
			fatal(err)
		}

		err = lockfile.Save(lockFile)
		if err != nil {
			fatal(err)
		}

		inst, err := installer.New(lookup.Type)
		if err != nil {
			fatal(err)
		}
		err = inst.Install(m.AddonsPath, lookup.Name, lockFile.Packages[lookup.Name])
		if err != nil {
			fatal(err)
		}

		fmt.Printf("Added %s@%s\n", lookup.Name, lookup.Version)
	},
}

func init() {
	rootCmd.AddCommand(requireCmd)
}
