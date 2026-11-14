package lockfile

import (
	"slices"
	"testing"
)

func TestPlan(t *testing.T) {
	v1 := LockedPackage{Version: "19.0.1.0.0", Dist: Dist{Type: "zip", URL: "https://example.com/1.zip", Shasum: "sha256:01"}}
	v2 := LockedPackage{Version: "19.0.2.0.0", Dist: Dist{Type: "zip", URL: "https://example.com/2.zip", Shasum: "sha256:02"}}
	moved := v1
	moved.Dist.Shasum = "sha256:03"
	reindexed := v1
	reindexed.Index = "company"
	reindexed.Direct = true
	reindexed.Depends = []string{"acme/base-tools"}

	tests := []struct {
		name          string
		prev, next    map[string]LockedPackage
		install, drop []string
	}{
		{"nothing locked", nil, nil, nil, nil},
		{"unchanged", map[string]LockedPackage{"acme/a": v1}, map[string]LockedPackage{"acme/a": v1}, nil, nil},
		{"new package", nil, map[string]LockedPackage{"acme/a": v1}, []string{"acme/a"}, nil},
		{"new version", map[string]LockedPackage{"acme/a": v1}, map[string]LockedPackage{"acme/a": v2}, []string{"acme/a"}, nil},
		{"same version, other dist", map[string]LockedPackage{"acme/a": v1}, map[string]LockedPackage{"acme/a": moved}, []string{"acme/a"}, nil},
		{"only metadata changed", map[string]LockedPackage{"acme/a": v1}, map[string]LockedPackage{"acme/a": reindexed}, nil, nil},
		{"dropped", map[string]LockedPackage{"acme/a": v1, "acme/b": v1}, map[string]LockedPackage{"acme/a": v1}, nil, []string{"acme/b"}},
		{
			"mixed, sorted",
			map[string]LockedPackage{"acme/d": v1, "acme/c": v1, "acme/b": v1, "acme/a": v1},
			map[string]LockedPackage{"acme/f": v1, "acme/e": v1, "acme/b": v2, "acme/a": v1},
			[]string{"acme/b", "acme/e", "acme/f"},
			[]string{"acme/c", "acme/d"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			install, drop := Plan(tc.prev, tc.next)
			if !slices.Equal(install, tc.install) || !slices.Equal(drop, tc.drop) {
				t.Errorf("Plan() = %v, %v, want %v, %v", install, drop, tc.install, tc.drop)
			}
		})
	}
}

func TestDependents(t *testing.T) {
	packages := map[string]LockedPackage{
		"acme/c": {Depends: []string{"acme/base-tools"}},
		"acme/a": {Depends: []string{"acme/x", "acme/base-tools"}},
		"acme/b": {Depends: []string{"acme/x"}},
	}
	if got := Dependents(packages, "acme/base-tools"); !slices.Equal(got, []string{"acme/a", "acme/c"}) {
		t.Errorf("Dependents() = %v", got)
	}
	if got := Dependents(packages, "acme/a"); got != nil {
		t.Errorf("Dependents() = %v, want none", got)
	}
}
