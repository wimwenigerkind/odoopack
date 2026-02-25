/*
Copyright © 2026 Wim
*/
package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
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
			fmt.Println("no addons installed")
			return
		}

		data := pterm.TableData{{"Name", "Version", "Installed"}}
		for name, version := range m.Require {
			data = append(data, []string{name, version, "true"})
		}

		table := pterm.DefaultTable.WithHasHeader().WithData(data)
		table.Render()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
