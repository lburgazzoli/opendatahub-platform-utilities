package predicate

import (
	crpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
)

// DefaultPredicate accepts meaningful events for a secondary resource while
// retaining controller-runtime metadata predicate defaults.
//
//nolint:gochecknoglobals // The predicate is immutable and stateless.
var DefaultPredicate = crpredicate.Or(
	GenerationChangedOnUpdate(),
	crpredicate.LabelChangedPredicate{},      //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
	crpredicate.AnnotationChangedPredicate{}, //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
)

// DefaultDeploymentPredicate also observes deployment availability changes.
//
//nolint:gochecknoglobals // The predicate is immutable and stateless.
var DefaultDeploymentPredicate = crpredicate.Or(
	DeploymentStatusChanged(),
	crpredicate.LabelChangedPredicate{},      //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
	crpredicate.AnnotationChangedPredicate{}, //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
)

// PartOf accepts updates and deletes for resources belonging to one primary
// kind. It is used by non-owned Watches defaults.
func PartOf(kind string) crpredicate.Predicate {
	return crpredicate.And(DefaultPredicate, LabelFor(labels.PlatformPartOf, kind))
}
