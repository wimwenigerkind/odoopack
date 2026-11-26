/*
Copyright © 2026 Wim
*/
package cmd

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List required addons",
	Aliases: []string{"l"},
	Run: func(cmd *cobra.Command, args []string) {
		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		lock, err := lockfile.LoadOrNew()
		if err != nil {
			fatal(err)
		}

		if len(m.Require) == 0 {
			ui.Info("no addons required")
			return
		}

		names := make([]string, 0, len(m.Require))
		for name := range m.Require {
			names = append(names, name)
		}
		sort.Strings(names)

		var rows [][]string
		for _, name := range names {
			rows = append(rows, []string{name, m.Require[name], installedStatus(m.AddonsDir, lock.Packages[name].Module)})
		}

		ui.Println(ui.Table([]string{"Name", "Version", "Installed"}, rows))
	},
}

func installedStatus(addonsDir, module string) string {
	if module == "" {
		return "no"
	}
	info, err := os.Stat(filepath.Join(addonsDir, module))
	if err == nil && info.IsDir() {
		return "yes"
	}
	return "no"
}

func init() {
	rootCmd.AddCommand(listCmd)
}
