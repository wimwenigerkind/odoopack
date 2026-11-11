/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/buildinfo"
)

func SetBuildInfo(version, commit, date string) {
	if version != "" {
		buildinfo.Version = version
	}
	if commit != "" {
		buildinfo.Commit = commit
	}
	if date != "" {
		buildinfo.Date = date
	}
	rootCmd.Version = buildinfo.Version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("odoopack %s\n", buildinfo.Version)
		fmt.Printf("  commit: %s\n", buildinfo.Commit)
		fmt.Printf("  built:  %s\n", buildinfo.Date)
		fmt.Printf("  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.Version = buildinfo.Version
	rootCmd.SetVersionTemplate("odoopack {{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
}
