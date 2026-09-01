/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

func SetBuildInfo(version, commit, date string) {
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
	if date != "" {
		buildDate = date
	}
	rootCmd.Version = buildVersion
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("odoopack %s\n", buildVersion)
		fmt.Printf("  commit: %s\n", buildCommit)
		fmt.Printf("  built:  %s\n", buildDate)
		fmt.Printf("  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.Version = buildVersion
	rootCmd.SetVersionTemplate("odoopack {{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
}
