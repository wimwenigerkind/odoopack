/*
Copyright © 2026 Wim Wenigerkind
*/
package cmd

import (
	"sort"

	"github.com/charmbracelet/lipgloss/tree"
	"github.com/spf13/cobra"
	"github.com/wimwenigerkind/odoopack/pkg/lockfile"
	"github.com/wimwenigerkind/odoopack/pkg/manifest"
	"github.com/wimwenigerkind/odoopack/pkg/ui"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Show the resolved dependency tree",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		m, err := manifest.Load()
		if err != nil {
			fatal(err)
		}

		lock := lockfile.LoadOrNew()
		if len(lock.Packages) == 0 {
			ui.Info("no dependencies (run 'odoopack install')")
			return
		}

		directs := make([]string, 0)
		for name, pkg := range lock.Packages {
			if pkg.Direct {
				directs = append(directs, name)
			}
		}
		sort.Strings(directs)
		if len(directs) == 0 {
			for name := range lock.Packages {
				directs = append(directs, name)
			}
			sort.Strings(directs)
		}

		expanded := make(map[string]bool)
		root := tree.Root(ui.Heading(m.Name))
		for _, name := range directs {
			root.Child(buildDepNode(name, lock.Packages, expanded))
		}
		ui.Println(root.String())
	},
}

func buildDepNode(name string, pkgs map[string]lockfile.LockedPackage, expanded map[string]bool) *tree.Tree {
	pkg, ok := pkgs[name]
	if !ok {
		return tree.Root(ui.Accent(name) + " " + ui.Muted("(missing)"))
	}
	label := ui.Accent(name) + ui.Muted("@"+pkg.Version)
	if expanded[name] {
		return tree.Root(label + " " + ui.Muted("(↑)"))
	}
	expanded[name] = true

	node := tree.Root(label)

	deps := append([]string(nil), pkg.Depends...)
	sort.Strings(deps)
	for _, d := range deps {
		node.Child(buildDepNode(d, pkgs, expanded))
	}

	ext := append([]string(nil), pkg.External...)
	sort.Strings(ext)
	for _, e := range ext {
		node.Child(ui.Muted(e + " (external)"))
	}

	return node
}

func init() {
	rootCmd.AddCommand(treeCmd)
}
