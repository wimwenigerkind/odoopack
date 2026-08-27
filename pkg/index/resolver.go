package index

import (
	"fmt"

	odoosemver "github.com/wimwenigerkind/odoopack-semver"
)

func resolveVersion(constraint, odooSeries string, candidates []registryVersion) (*registryVersion, error) {
	c, err := odoosemver.ParseConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", constraint, err)
	}

	parsed := make([]odoosemver.Version, 0, len(candidates))
	byVersion := make(map[string]int, len(candidates))
	for i := range candidates {
		v, err := odoosemver.Parse(candidates[i].Version)
		if err != nil {
			continue
		}
		parsed = append(parsed, v)
		byVersion[v.String()] = i
	}

	best, ok := odoosemver.Resolve(c, parsed)
	if !ok {
		return nil, fmt.Errorf("no version matches %q", constraint)
	}

	if odooSeries != "" && best.IsRelease() && best.SeriesString() != odooSeries {
		return nil, fmt.Errorf("version %s targets Odoo %s but the project targets %s", best.String(), best.SeriesString(), odooSeries)
	}

	return &candidates[byVersion[best.String()]], nil
}
