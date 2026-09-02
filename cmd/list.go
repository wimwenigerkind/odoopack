/*
Copyright © 2026 Wim
*/
package cmd

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/installer"
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
			rows = append(rows, []string{name, m.Require[name], installedStatus(m.AddonsPath, name)})
		}

		ui.Println(ui.Table([]string{"Name", "Version", "Installed"}, rows))
	},
}

func installedStatus(addonsPath, name string) string {
	info, err := os.Stat(filepath.Join(addonsPath, installer.FormatAddonDir(name)))
	if err == nil && info.IsDir() {
		return "yes"
	}
	return "no"
}

func init() {
	rootCmd.AddCommand(listCmd)
}
