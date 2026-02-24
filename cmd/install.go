/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
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
			fmt.Println("no addons installed")
			return
		}

		lock := lockfile.LoadOrNew()

		isStale, err := lockfile.IsStale(m.Require, lock.ContentHash)
		if err != nil {
			fatal(err)
		}

		if isStale {
			fmt.Println("lockfile is stale")
			lock, err = lockfile.RecomputeHash(m.Require, m.Indexes)
			if err != nil {
				fatal(err)
			}

			err = lockfile.Save(lock)
			if err != nil {
				fatal(err)
			}
		}

		if err = os.RemoveAll(m.AddonsPath); err != nil {
			fatal(err)
		}

		fmt.Println("Install")

		for name, lockedPackage := range lock.Packages {
			fmt.Printf("cloning %s@%s\n", name, lockedPackage.Version)
			inst, err := installer.New(lockedPackage.Type)
			if err != nil {
				fatal(err)
			}

			err = inst.Install(m.AddonsPath, name, lockedPackage)
			if err != nil {
				fatal(err)
			}
			fmt.Printf("cloned %s@%s\n", name, lockedPackage.Version)
		}

		fmt.Println("Installed")
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
