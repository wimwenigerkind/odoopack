/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

var name string
var odoo string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new odoopack project",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m, err := manifest.Init(name, odoo)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Initialized project %q\n", m.Name)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&name, "name", "n", "odoopack", "Project name")
	initCmd.Flags().StringVar(&odoo, "odoo", "19.0", "Target Odoo series")
}
