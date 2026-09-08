package cluster

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/openshift"
)

const (
	// Kubernetes identifies a standard Kubernetes distribution.
	Kubernetes api.DistributionKind = "Kubernetes"
	// OpenShift identifies the OpenShift Kubernetes distribution.
	OpenShift api.DistributionKind = "OpenShift"
)

// DetectClusterDistribution identifies the Kubernetes distribution exposed by
// the cluster. OpenShift API absence is a valid Kubernetes result; a missing
// required OpenShift singleton is returned as an error.
func DetectClusterDistribution(ctx context.Context, reader client.Reader) (api.Distribution, error) {
	version, err := openshift.GetVersion(ctx, reader)
	if err == nil {
		return api.Distribution{Kind: OpenShift, Version: version}, nil
	}

	switch {
	case meta.IsNoMatchError(err):
		return api.Distribution{Kind: Kubernetes}, nil
	case apierrors.IsNotFound(err):
		return api.Distribution{}, fmt.Errorf("detect cluster distribution: %w", err)
	default:
		return api.Distribution{}, fmt.Errorf("detect cluster distribution: %w", err)
	}
}
