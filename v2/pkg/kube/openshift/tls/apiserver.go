package tls

import (
	"context"
	"errors"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadResult contains the startup profile and whether the API should be
// watched for a profile that may become available or change later.
type LoadResult struct {
	Spec      configv1.TLSProfileSpec
	Watchable bool
}

// FetchAPIServerTLSProfile reads the OpenShift APIServer TLS profile.
func FetchAPIServerTLSProfile(ctx context.Context, reader client.Reader) (configv1.TLSProfileSpec, error) {
	apiServer := &configv1.APIServer{}

	err := reader.Get(ctx, client.ObjectKey{Name: APIServerName}, apiServer)
	if err != nil {
		return configv1.TLSProfileSpec{}, fmt.Errorf("get APIServer %q: %w", APIServerName, err)
	}

	return *ProfileSpecFromSecurityProfile(apiServer.Spec.TLSSecurityProfile), nil
}

// FromAPIServer returns proxy-compatible TLS settings. A missing OpenShift
// API falls back to the Intermediate profile for vanilla Kubernetes.
func FromAPIServer(
	ctx context.Context,
	reader client.Reader,
	format VersionFormat,
) (string, string, error) {
	apiServer := &configv1.APIServer{}
	err := reader.Get(ctx, client.ObjectKey{Name: APIServerName}, apiServer)
	if err == nil {
		minVersion, cipherSuites := FromProfile(ctx, apiServer.Spec.TLSSecurityProfile, format)
		return minVersion, cipherSuites, nil
	}

	switch {
	case apierrors.IsNotFound(err):
		minVersion, cipherSuites := FromProfile(ctx, nil, format)
		return minVersion, cipherSuites, nil
	case meta.IsNoMatchError(err):
		minVersion, cipherSuites := FromProfile(ctx, nil, format)
		return minVersion, cipherSuites, nil
	default:
		return "", "", fmt.Errorf("get APIServer %q: %w", APIServerName, err)
	}
}

// Load resolves the startup TLS profile and whether a profile watcher should
// be registered.
func Load(ctx context.Context, reader client.Reader) (LoadResult, error) {
	spec, err := FetchAPIServerTLSProfile(ctx, reader)
	if err == nil {
		return LoadResult{Spec: spec, Watchable: true}, nil
	}

	switch {
	case apierrors.IsNotFound(err):
		return LoadResult{Spec: intermediateSpec()}, nil
	case meta.IsNoMatchError(err):
		return LoadResult{Spec: intermediateSpec()}, nil
	case errors.Is(err, context.DeadlineExceeded):
		return LoadResult{Spec: intermediateSpec(), Watchable: true}, nil
	case apierrors.IsServiceUnavailable(err):
		return LoadResult{Spec: intermediateSpec(), Watchable: true}, nil
	case apierrors.IsTimeout(err):
		return LoadResult{Spec: intermediateSpec(), Watchable: true}, nil
	case apierrors.IsServerTimeout(err):
		return LoadResult{Spec: intermediateSpec(), Watchable: true}, nil
	case apierrors.IsTooManyRequests(err):
		return LoadResult{Spec: intermediateSpec(), Watchable: true}, nil
	default:
		return LoadResult{}, err
	}
}
