/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/index"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

var requireCmd = &cobra.Command{
	Use:   "require [addon]@[version]",
	Short: "Add an addon dependency",
	Args:  cobra.RangeArgs(1, 2),
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
			fmt.Println(err)
			os.Exit(1)
		}

		indexProvider := index.StaticProvider{
			Endpoint: m.Indexes["default"].Url,
		}

		lookup, err := indexProvider.Lookup(addonName, version)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := m.AddRequirement(lookup.Name, lookup.Version); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		lockFile := lockfile.LoadOrNew()

		lockFile.Packages[lookup.Name] = lockfile.LockedPackage{
			Version:    lookup.Version,
			Type:       lookup.Type,
			Repository: lookup.Repository,
		}

		lockFile.ContentHash, err = lockfile.ComputeHash(m.Require)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = lockfile.Save(lockFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		inst, err := installer.New(lookup.Type)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = inst.Install(m.AddonsPath, lookup.Name, lockFile.Packages[lookup.Name])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Added %s@%s\n", lookup.Name, lookup.Version)
	},
}

func init() {
	rootCmd.AddCommand(requireCmd)
}
