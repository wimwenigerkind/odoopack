/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/addonsdir"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the project manifest and lockfile for problems",
	Run: func(cmd *cobra.Command, args []string) {
		var errs, warns int
		ok := func(format string, a ...any) { ui.Success(format, a...) }
		warn := func(format string, a ...any) { warns++; ui.Warn(format, a...) }
		bad := func(format string, a ...any) { errs++; ui.Error(format, a...) }

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		if m.Name != "" {
			ok("name: %s", m.Name)
		}

		ok("odoo series: %s", m.OdooSeries)

		if wd, err := os.Getwd(); err != nil {
			bad("addons_dir: %v", err)
		} else if err := addonsdir.Check(wd, m.AddonsDir); err != nil {
			bad("%v", err)
		} else {
			ok("addons_dir: %s", m.AddonsDir)
		}

		if len(m.Indexes) == 0 {
			warn("no indexes configured, so no addon can be resolved")
		} else {
			ok("%d index(es) configured", len(m.Indexes))
		}

		if lf, err := lockfile.Load(); err != nil {
			warn("no lockfile found (run 'odoopack install')")
		} else if stale, err := lockfile.IsStale(m.Require, m.Indexes, m.OdooSeries, lf.Packages, lf.ContentHash); err == nil && stale {
			warn("lockfile is out of date (run 'odoopack install')")
		} else {
			ok("lockfile up to date (%d package(s))", len(lf.Packages))
		}

		ui.Println()
		switch {
		case errs > 0:
			ui.Error("%d error(s), %d warning(s)", errs, warns)
			os.Exit(1)
		case warns > 0:
			ui.Warn("no errors, %d warning(s)", warns)
		default:
			ui.Success("all checks passed")
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
