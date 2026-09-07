package release

import "github.com/opendatahub-io/odh-platform-utilities/v2/api"

const Platform = "platform"

func Get(accessor api.ReleaseStatusAccessor, name string) *api.ComponentRelease {
	if accessor == nil || accessor.GetReleaseStatus() == nil {
		return nil
	}

	for _, value := range accessor.GetReleaseStatus().Releases {
		if value.Name == name {
			return value.DeepCopy()
		}
	}

	return nil
}

func Set(accessor api.ReleaseStatusAccessor, value api.ComponentRelease) bool {
	if accessor == nil {
		return false
	}

	status := accessor.GetReleaseStatus()
	if status == nil {
		return false
	}

	updated := *status.DeepCopy()
	for index := range updated.Releases {
		if updated.Releases[index].Name == value.Name {
			if updated.Releases[index] == value {
				return false
			}

			updated.Releases[index] = value
			accessor.SetReleaseStatus(updated)

			return true
		}
	}

	updated.Releases = append(updated.Releases, value)
	accessor.SetReleaseStatus(updated)

	return true
}

func PlatformVersion(accessor api.ReleaseStatusAccessor) string {
	value := Get(accessor, Platform)
	if value == nil {
		return ""
	}

	return value.Version
}

func SetPlatformVersion(accessor api.ReleaseStatusAccessor, version string) bool {
	return Set(accessor, api.ComponentRelease{Name: Platform, Version: version})
}
