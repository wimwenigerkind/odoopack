/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"golang.org/x/sync/errgroup"
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

		isStale, err := lockfile.IsStale(m.Require, m.Indexes, lock.Packages, lock.ContentHash)
		if err != nil {
			fatal(err)
		}

		if isStale {
			fmt.Println("lockfile is stale")
			lock, err = lockfile.RecomputeHash(m.Require, m.Indexes, m.Odoo)
			if err != nil {
				fatal(err)
			}
			if err := lockfile.Save(lock); err != nil {
				fatal(err)
			}
		}

		if err := installAll(m, lock); err != nil {
			fmt.Println("error while installing:", err)
			os.Exit(1)
		}
	},
}

func installAll(m *manifest.Manifest, lock lockfile.LockFile) error {
	if err := os.RemoveAll(m.AddonsPath); err != nil {
		return err
	}

	multi := pterm.DefaultMultiPrinter

	type job struct {
		name    string
		pkg     lockfile.LockedPackage
		spinner *pterm.SpinnerPrinter
	}

	jobs := make([]job, 0, len(lock.Packages))
	for name, lockedPackage := range lock.Packages {
		spinner, _ := pterm.DefaultSpinner.WithWriter(multi.NewWriter()).Start("installing " + name + "@" + lockedPackage.Version)
		jobs = append(jobs, job{name: name, pkg: lockedPackage, spinner: spinner})
	}

	multi.Start()
	defer multi.Stop()

	var eg errgroup.Group
	for _, j := range jobs {
		eg.Go(func() error {
			inst, err := installer.New(j.pkg.Dist.Type)
			if err != nil {
				j.spinner.Fail()
				return fmt.Errorf("%s: %w", j.name, err)
			}
			if err := inst.Install(m.AddonsPath, j.name, j.pkg); err != nil {
				j.spinner.Fail()
				return fmt.Errorf("%s: %w", j.name, err)
			}
			j.spinner.Success()
			return nil
		})
	}
	return eg.Wait()
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
