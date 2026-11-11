/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/store"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the shared package store",
}

var cacheDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Print the cache directory",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := store.CacheDir()
		if err != nil {
			fatal(err)
		}
		fmt.Println(dir)
	},
}

var cachePruneAll bool

var cachePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove packages no project uses any more",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		st, err := store.Open()
		if err != nil {
			fatal(err)
		}
		res, err := st.Prune(cachePruneAll)
		if err != nil {
			fatal(err)
		}
		ui.Success("removed %d package(s), freed %s", res.Entries, formatBytes(res.Bytes))
	},
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func init() {
	cachePruneCmd.Flags().BoolVar(&cachePruneAll, "all", false, "remove every package, including those still in use")
	cacheCmd.AddCommand(cacheDirCmd, cachePruneCmd)
	rootCmd.AddCommand(cacheCmd)
}
