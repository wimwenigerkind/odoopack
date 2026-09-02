/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
)

var indexCmd = &cobra.Command{
	Use:     "index",
	Short:   "Manage addon registry indexes",
	Aliases: []string{"idx"},
}

var indexAddType string

var indexAddCmd = &cobra.Command{
	Use:   "add [name] [url]",
	Short: "Add or replace a registry index",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		raw := args[1]

		if name == "" {
			fatal(fmt.Errorf("index name must not be empty"))
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			fatal(fmt.Errorf("invalid url %q", raw))
		}

		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}
		if m.Indexes == nil {
			m.Indexes = manifest.Indexes{}
		}
		m.Indexes[name] = manifest.Index{Url: raw, Type: indexAddType}
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}
		pterm.Success.Printfln("added index %s %s (%s)", name, raw, indexAddType)
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
		if _, ok := m.Indexes[name]; !ok {
			fatal(fmt.Errorf("index %q is not declared", name))
		}
		delete(m.Indexes, name)
		if err := manifest.Save(*m); err != nil {
			fatal(err)
		}
		pterm.Success.Printfln("removed index %s", name)
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

		data := pterm.TableData{{"Name", "URL", "Type", "Source"}}

		names := make([]string, 0, len(m.Indexes))
		for n := range m.Indexes {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			idx := m.Indexes[n]
			data = append(data, []string{n, idx.Url, idx.Type, "manifest"})
		}

		defaultURL := viper.GetString("default_index_url")
		if defaultURL != "" && shouldShowImplicitDefault(m.Indexes, defaultURL) {
			data = append(data, []string{"default", defaultURL, "registry", "implicit"})
		}

		if len(data) == 1 {
			pterm.Info.Println("no indexes configured")
			return
		}

		_ = pterm.DefaultTable.WithHasHeader().WithData(data).WithBoxed().Render()
	},
}

func shouldShowImplicitDefault(indexes manifest.Indexes, defaultURL string) bool {
	if _, hasDefault := indexes["default"]; hasDefault {
		return false
	}
	for _, idx := range indexes {
		if idx.Url == defaultURL {
			return false
		}
	}
	return true
}

func init() {
	indexAddCmd.Flags().StringVar(&indexAddType, "type", "registry", "index type")
	indexCmd.AddCommand(indexAddCmd)
	indexCmd.AddCommand(indexRemoveCmd)
	indexCmd.AddCommand(indexListCmd)
	rootCmd.AddCommand(indexCmd)
}
