/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var indexCmd = &cobra.Command{
	Use:     "index",
	Short:   "Manage addon registry indexes",
	Aliases: []string{"idx"},
}

var (
	indexAddType   string
	indexAddAppend bool
)

var indexAddCmd = &cobra.Command{
	Use:   "add [name] [url]",
	Short: "Add or replace a registry index",
	Long: `Add a registry index in front of the others, so that it is searched first,
or with --append behind them. An index with the same name is replaced where it
is.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		raw := args[1]

		idx := manifest.Index{Name: name, Url: raw, Type: indexAddType}
		if err := idx.Validate(); err != nil {
			fatal(err)
		}

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		m.Indexes = m.Indexes.Add(idx, indexAddAppend)
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}
		ui.Success("added index %s %s (%s)", name, raw, indexAddType)
	},
}

var indexRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Short:   "Remove a registry index",
	Aliases: []string{"rm", "delete"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		var removed bool
		if m.Indexes, removed = m.Indexes.Remove(name); !removed {
			fatal(fmt.Errorf("index %q is not declared", name))
		}
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}
		ui.Success("removed index %s", name)
	},
}

var indexListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List configured registry indexes",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		if len(m.Indexes) == 0 {
			ui.Info("no indexes configured")
			return
		}

		var rows [][]string
		for i, idx := range m.Indexes {
			rows = append(rows, []string{strconv.Itoa(i + 1), idx.Name, idx.Url, idx.Type})
		}
		ui.Println(ui.Table([]string{"#", "Name", "URL", "Type"}, rows))
	},
}

func init() {
	indexAddCmd.Flags().StringVar(&indexAddType, "type", "registry", "index type")
	indexAddCmd.Flags().BoolVar(&indexAddAppend, "append", false, "search the index after the others instead of first")
	indexCmd.AddCommand(indexAddCmd)
	indexCmd.AddCommand(indexRemoveCmd)
	indexCmd.AddCommand(indexListCmd)
	rootCmd.AddCommand(indexCmd)
}
