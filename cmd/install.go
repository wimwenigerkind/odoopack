/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"
	"sync"

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

		fmt.Println("Installing")

		var wg sync.WaitGroup

		errChan := make(chan error, len(lock.Packages))

		for name, lockedPackage := range lock.Packages {
			wg.Add(1)
			go func(name string, pkg lockfile.LockedPackage) {
				defer wg.Done()
				fmt.Printf("installing %s@%s\n", name, pkg.Version)

				inst, err := installer.New(pkg.Type)
				if err != nil {
					errChan <- err
					return
				}

				err = inst.Install(m.AddonsPath, name, pkg)
				if err != nil {
					errChan <- err
				}
			}(name, lockedPackage)
		}

		wg.Wait()
		close(errChan)

		if len(errChan) > 0 {
			fmt.Println("errors occurred, quitting:")
			for err := range errChan {
				fmt.Println("error while installing:", err)
			}
			os.Exit(1)
		}

		fmt.Println("Installed")
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
