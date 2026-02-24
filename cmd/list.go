/*
Copyright © 2026 Wim
*/
package cmd

import (
	"fmt"

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

		fmt.Println("Installed")
		for name, version := range m.Require {
			fmt.Println("-", name+"@"+version)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
