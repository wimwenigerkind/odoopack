/*
Copyright © 2026 Wim
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var removeCmd = &cobra.Command{
	Use:   "remove [addon]",
	Short: "Remove a addon from requirements",
	Args:  cobra.RangeArgs(1, 1),
	Run: func(cmd *cobra.Command, args []string) {
		addon := args[0]
		addonParts := strings.Split(addon, "@")
		addonName := addonParts[0]

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		if len(m.Require) == 0 {
			ui.Info("no addons required")
			return
		}

		version, ok := m.Require[addonName]
		if !ok {
			fatal(fmt.Errorf("addon is not installed"))
		}

		m.RemoveRequirement(addonName)
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}

		lock := lockfile.LoadOrNew()
		delete(lock.Packages, addonName)

		lock.ContentHash, err = lockfile.ComputeHash(m.Require, m.Indexes, lock.Packages)
		if err != nil {
			fatal(err)
		}

		err = lockfile.Save(lock)
		if err != nil {
			fatal(err)
		}

		addonDir := installer.FormatAddonDir(addonName)
		err = os.RemoveAll(filepath.Join(m.AddonsPath, addonDir))
		if err != nil {
			fatal(err)
		}

		ui.Success("removed %s@%s", addonName, version)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
