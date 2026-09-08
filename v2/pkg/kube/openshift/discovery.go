package openshift

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
)

const (
	// AuthenticationIntegratedOAuth is the default OpenShift authentication mode.
	AuthenticationIntegratedOAuth = "IntegratedOAuth"
	// AuthenticationOIDC identifies external OIDC authentication.
	AuthenticationOIDC = "OIDC"
	// AuthenticationNone identifies disabled authentication.
	AuthenticationNone = "None"
)

const (
	fipsConfigMapName      = "cluster-config-v1"
	fipsConfigMapNamespace = "kube-system"
)

type installConfig struct {
	FIPS bool `json:"fips"`
}

// GetVersion returns the OpenShift version from ClusterVersion status.
func GetVersion(ctx context.Context, reader client.Reader) (string, error) {
	clusterVersion := new(unstructured.Unstructured)
	clusterVersion.SetGroupVersionKind(gvk.ClusterVersion)

	if err := reader.Get(ctx, client.ObjectKey{Name: "version"}, clusterVersion); err != nil {
		return "", fmt.Errorf("get OpenShift version: %w", err)
	}

	history, found, err := unstructured.NestedSlice(clusterVersion.Object, "status", "history")
	if err != nil {
		return "", fmt.Errorf("read OpenShift version history: %w", err)
	}
	if !found || len(history) == 0 {
		return "", fmt.Errorf("OpenShift version history is empty")
	}

	entry, ok := history[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("OpenShift version history entry is not an object")
	}

	version, ok := entry["version"].(string)
	if !ok || version == "" {
		return "", fmt.Errorf("OpenShift version is empty")
	}

	return version, nil
}

// IsFIPSEnabled reads the installer ConfigMap used by OpenShift. An absent
// ConfigMap is treated as false for clusters where installer data is absent.
func IsFIPSEnabled(ctx context.Context, reader client.Reader) (bool, error) {
	configMap := new(unstructured.Unstructured)
	configMap.SetGroupVersionKind(gvk.ConfigMap)

	err := reader.Get(ctx, types.NamespacedName{
		Namespace: fipsConfigMapNamespace,
		Name:      fipsConfigMapName,
	}, configMap)
	if err != nil {
		switch {
		case apierrors.IsNotFound(err):
			return false, nil
		case meta.IsNoMatchError(err):
			return false, nil
		default:
			return false, fmt.Errorf("read cluster FIPS configuration: %w", err)
		}
	}

	data, found, err := unstructured.NestedStringMap(configMap.Object, "data")
	if err != nil {
		return false, fmt.Errorf("read cluster FIPS configuration data: %w", err)
	}
	if !found || data["install-config"] == "" {
		return false, nil
	}

	var configuration installConfig
	if err := yaml.Unmarshal([]byte(data["install-config"]), &configuration); err != nil {
		return false, fmt.Errorf("parse cluster FIPS configuration: %w", err)
	}

	return configuration.FIPS, nil
}

// IsSingleNodeCluster reports whether OpenShift has a single-replica control
// plane, falling back to the number of schedulable nodes when Infrastructure
// is unavailable.
func IsSingleNodeCluster(ctx context.Context, reader client.Reader) (bool, error) {
	infrastructure := new(unstructured.Unstructured)
	infrastructure.SetGroupVersionKind(gvk.Infrastructure)

	err := reader.Get(ctx, client.ObjectKey{Name: "cluster"}, infrastructure)
	if err == nil {
		topology, _, topologyErr := unstructured.NestedString(
			infrastructure.Object,
			"status",
			"controlPlaneTopology",
		)
		if topologyErr != nil {
			return false, fmt.Errorf("read control-plane topology: %w", topologyErr)
		}

		return topology == "SingleReplica", nil
	}

	switch {
	case apierrors.IsNotFound(err):
		return isSingleNodeBySchedulableNodes(ctx, reader)
	case meta.IsNoMatchError(err):
		return isSingleNodeBySchedulableNodes(ctx, reader)
	default:
		return false, fmt.Errorf("get OpenShift infrastructure: %w", err)
	}
}

func isSingleNodeBySchedulableNodes(ctx context.Context, reader client.Reader) (bool, error) {
	nodes := new(unstructured.UnstructuredList)
	nodes.SetGroupVersionKind(gvk.Node)

	if err := reader.List(ctx, nodes); err != nil {
		return false, fmt.Errorf("list nodes: %w", err)
	}

	schedulable := 0
	for _, node := range nodes.Items {
		unschedulable, _, err := unstructured.NestedBool(node.Object, "spec", "unschedulable")
		if err != nil {
			return false, fmt.Errorf("read node schedulability: %w", err)
		}
		if !unschedulable {
			schedulable++
		}
	}

	return schedulable == 1, nil
}

// GetAuthenticationMode returns the OpenShift Authentication type.
func GetAuthenticationMode(ctx context.Context, reader client.Reader) (string, error) {
	authentication := new(unstructured.Unstructured)
	authentication.SetGroupVersionKind(gvk.Authentication)

	if err := reader.Get(ctx, client.ObjectKey{Name: "cluster"}, authentication); err != nil {
		return "", fmt.Errorf("get OpenShift authentication: %w", err)
	}

	authenticationType, found, err := unstructured.NestedString(
		authentication.Object,
		"spec",
		"type",
	)
	if err != nil {
		return "", fmt.Errorf("read OpenShift authentication type: %w", err)
	}
	if !found || authenticationType == "" {
		return AuthenticationIntegratedOAuth, nil
	}

	switch authenticationType {
	case AuthenticationOIDC:
		return AuthenticationOIDC, nil
	case AuthenticationNone:
		return AuthenticationNone, nil
	case AuthenticationIntegratedOAuth:
		return AuthenticationIntegratedOAuth, nil
	default:
		return AuthenticationNone, nil
	}
}

// IsIntegratedOAuth reports whether OpenShift uses integrated OAuth.
func IsIntegratedOAuth(ctx context.Context, reader client.Reader) (bool, error) {
	mode, err := GetAuthenticationMode(ctx, reader)
	if err != nil {
		return false, err
	}

	return mode == AuthenticationIntegratedOAuth, nil
}

// GetServiceAccountIssuer returns the configured OpenShift service-account
// issuer, preserving NotFound and NoMatch errors for vanilla clusters.
func GetServiceAccountIssuer(ctx context.Context, reader client.Reader) (string, error) {
	authentication := new(unstructured.Unstructured)
	authentication.SetGroupVersionKind(gvk.Authentication)

	if err := reader.Get(ctx, client.ObjectKey{Name: "cluster"}, authentication); err != nil {
		return "", err
	}

	issuer, found, err := unstructured.NestedString(
		authentication.Object,
		"spec",
		"serviceAccountIssuer",
	)
	if err != nil {
		return "", fmt.Errorf("read service-account issuer: %w", err)
	}
	if !found {
		return "", nil
	}

	return issuer, nil
}

// GetDomain returns the OpenShift application domain, preferring appsDomain.
func GetDomain(ctx context.Context, reader client.Reader) (string, error) {
	ingress := new(unstructured.Unstructured)
	ingress.SetGroupVersionKind(gvk.Ingress)

	if err := reader.Get(ctx, client.ObjectKey{Name: "cluster"}, ingress); err != nil {
		return "", fmt.Errorf("get OpenShift ingress: %w", err)
	}

	appsDomain, found, err := unstructured.NestedString(ingress.Object, "spec", "appsDomain")
	if err != nil {
		return "", fmt.Errorf("read OpenShift apps domain: %w", err)
	}
	if found && appsDomain != "" {
		return appsDomain, nil
	}

	domain, found, err := unstructured.NestedString(ingress.Object, "spec", "domain")
	if err != nil {
		return "", fmt.Errorf("read OpenShift domain: %w", err)
	}
	if !found || domain == "" {
		return "", fmt.Errorf("OpenShift ingress domain is empty")
	}

	return domain, nil
}
