/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	odoosemver "github.com/wimwenigerkind/odoopack-semver"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

var seriesPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the project manifest and lockfile for problems",
	Run: func(cmd *cobra.Command, args []string) {
		var errs, warns int
		ok := func(format string, a ...any) { fmt.Printf("  ok     %s\n", fmt.Sprintf(format, a...)) }
		warn := func(format string, a ...any) { warns++; fmt.Printf("  warn   %s\n", fmt.Sprintf(format, a...)) }
		bad := func(format string, a ...any) { errs++; fmt.Printf("  error  %s\n", fmt.Sprintf(format, a...)) }

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		if m.Name == "" {
			bad("name is empty")
		} else {
			ok("name: %s", m.Name)
		}

		switch {
		case m.Odoo == "":
			bad("odoo series not set (add \"odoo\": \"19.0\" to odoopack.json)")
		case !seriesPattern.MatchString(m.Odoo):
			bad("odoo %q is not a valid series (expected like 19.0)", m.Odoo)
		default:
			ok("odoo series: %s", m.Odoo)
		}

		if m.AddonsPath == "" {
			warn("addons_path not set")
		} else {
			ok("addons_path: %s", m.AddonsPath)
		}

		if len(m.Indexes) == 0 {
			warn("no indexes configured (relying on the default index if set)")
		} else {
			ok("%d index(es) configured", len(m.Indexes))
		}
		for name, idx := range m.Indexes {
			if idx.Url == "" {
				bad("index %q has no url", name)
			}
			if idx.Type == "" {
				warn("index %q has no type", name)
			}
		}

		for name, constraint := range m.Require {
			if constraint == "" {
				bad("require %s has an empty constraint", name)
				continue
			}
			if _, err := odoosemver.ParseConstraint(constraint); err != nil {
				bad("require %s: invalid constraint %q (%v)", name, constraint, err)
				continue
			}
			if v, err := odoosemver.Parse(constraint); err == nil && v.IsRelease() && m.Odoo != "" && v.SeriesString() != m.Odoo {
				warn("require %s pins %s which targets Odoo %s, but the project is %s", name, constraint, v.SeriesString(), m.Odoo)
			}
		}

		if lf, err := lockfile.Load(); err != nil {
			warn("no lockfile found (run 'odoopack install')")
		} else if stale, err := lockfile.IsStale(m.Require, m.Indexes, lf.Packages, lf.ContentHash); err == nil && stale {
			warn("lockfile is out of date (run 'odoopack install')")
		} else {
			ok("lockfile up to date (%d package(s))", len(lf.Packages))
		}

		fmt.Println()
		switch {
		case errs > 0:
			fmt.Printf("%d error(s), %d warning(s)\n", errs, warns)
			os.Exit(1)
		case warns > 0:
			fmt.Printf("no errors, %d warning(s)\n", warns)
		default:
			fmt.Println("all checks passed")
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
