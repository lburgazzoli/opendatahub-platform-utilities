package release

import (
	"cmp"
	"slices"
	"strings"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

func normalize(values []api.ComponentRelease) []api.ComponentRelease {
	result := make([]api.ComponentRelease, 0, len(values))
	for _, value := range values {
		value.Version = strings.TrimSpace(value.Version)
		if value.Version == "" {
			continue
		}

		result = append(result, value)
	}

	slices.SortFunc(result, func(left api.ComponentRelease, right api.ComponentRelease) int {
		return cmp.Compare(left.Name, right.Name)
	})

	return result
}
