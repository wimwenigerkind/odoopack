/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/pterm/pterm"
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

		var wg sync.WaitGroup

		errChan := make(chan error, len(lock.Packages))

		multi := pterm.DefaultMultiPrinter

		type job struct {
			name    string
			pkg     lockfile.LockedPackage
			spinner *pterm.SpinnerPrinter
		}

		var jobs []job
		for name, lockedPackage := range lock.Packages {
			spinner, _ := pterm.DefaultSpinner.WithWriter(multi.NewWriter()).Start("installing " + name + "@" + lockedPackage.Version)
			jobs = append(jobs, job{name: name, pkg: lockedPackage, spinner: spinner})
		}

		multi.Start()

		for _, j := range jobs {
			wg.Add(1)
			go func(j job) {
				defer wg.Done()

				inst, err := installer.New(j.pkg.Type)
				if err != nil {
					j.spinner.Fail()
					errChan <- err
					return
				}

				err = inst.Install(m.AddonsPath, j.name, j.pkg)
				if err != nil {
					j.spinner.Fail()
					errChan <- err
					return
				}
				j.spinner.Success()
			}(j)
		}

		wg.Wait()
		close(errChan)

		multi.Stop()

		if len(errChan) > 0 {
			fmt.Println("errors occurred, quitting:")
			for err := range errChan {
				fmt.Println("error while installing:", err)
			}
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
