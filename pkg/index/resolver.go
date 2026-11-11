package index

import (
	"fmt"

	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"github.com/wimwenigerkind/odoopack-spec/version"
)

func resolveVersion(constraint, odooSeries string, candidates []registryapi.Version) (*registryapi.Version, error) {
	c, err := version.ParseConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", constraint, err)
	}

	parsed := make([]version.Version, 0, len(candidates))
	byVersion := make(map[string]int, len(candidates))
	for i := range candidates {
		v, err := version.Parse(candidates[i].Version)
		if err != nil {
			continue
		}
		parsed = append(parsed, v)
		byVersion[v.String()] = i
	}

	best, ok := version.Resolve(c, parsed)
	if !ok {
		return nil, fmt.Errorf("no version matches %q", constraint)
	}

	if odooSeries != "" && best.IsRelease() && best.SeriesString() != odooSeries {
		return nil, fmt.Errorf("version %s targets Odoo %s but the project targets %s", best.String(), best.SeriesString(), odooSeries)
	}

	return &candidates[byVersion[best.String()]], nil
}
