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
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
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
			fmt.Println("no addons installed")
			return
		}

		if m.Require[addonName] == "" {
			fatal(fmt.Errorf("addon is not installed"))
		}
		version := m.Require[addonName]

		m.RemoveRequirement(addonName)
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}

		lock := lockfile.LoadOrNew()
		delete(lock.Packages, addonName)

		lock.ContentHash, err = lockfile.ComputeHash(m.Require)
		if err != nil {
			fatal(err)
		}

		err = lockfile.Save(lock)
		if err != nil {
			fatal(err)
		}

		addonDir := strings.ReplaceAll(addonName, "/", "_")
		err = os.RemoveAll(filepath.Join(m.AddonsPath, addonDir))
		if err != nil {
			fatal(err)
		}

		fmt.Println("removed", addonName+"@"+version)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
