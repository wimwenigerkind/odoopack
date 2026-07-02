package index

import "fmt"

func resolveVersion(constraint string, candidates []registryVersion) (*registryVersion, error) {
	for i := range candidates {
		if candidates[i].Version == constraint {
			return &candidates[i], nil
		}
	}
	return nil, fmt.Errorf("no version matches %q", constraint)
}
