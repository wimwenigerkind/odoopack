package lockfile

import "sort"

// Plan compares the packages locked before and after a change: install holds
// the packages of next that are new, locked to another version or dist, or
// moved to another module; remove the packages of prev that next no longer
// has or that moved, so that their old directory goes. Both are sorted.
func Plan(prev, next map[string]LockedPackage) (install, remove []string) {
	for name, pkg := range next {
		old, ok := prev[name]
		moved := ok && old.Module != pkg.Module
		if !ok || moved || old.Version != pkg.Version || !old.Dist.Same(pkg.Dist) {
			install = append(install, name)
		}
		if moved {
			remove = append(remove, name)
		}
	}
	for name := range prev {
		if _, ok := next[name]; !ok {
			remove = append(remove, name)
		}
	}
	sort.Strings(install)
	sort.Strings(remove)
	return install, remove
}

// Dependents returns the packages that depend on name, sorted.
func Dependents(packages map[string]LockedPackage, name string) []string {
	var dependents []string
	for dependent, pkg := range packages {
		for _, dep := range pkg.Depends {
			if dep == name {
				dependents = append(dependents, dependent)
				break
			}
		}
	}
	sort.Strings(dependents)
	return dependents
}
