package olm

import (
	"context"
	"errors"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
)

// ErrOperatorNotInstalled reports that no matching OperatorCondition exists.
var ErrOperatorNotInstalled = errors.New("operator not installed")

// ErrNamespaceRequired reports that a namespaced OLM lookup was given no
// namespace.
var ErrNamespaceRequired = errors.New("namespace is required")

// OperatorVersion returns the version suffix of an OLM OperatorCondition
// whose name starts with operatorPrefix and a dot.
func OperatorVersion(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	operatorPrefix string,
) (string, error) {
	if namespace == "" {
		return "", ErrNamespaceRequired
	}

	list := unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.OperatorCondition)

	if err := reader.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return "", err
	}

	prefix := operatorPrefix + "."
	for _, item := range list.Items {
		version, found := strings.CutPrefix(item.GetName(), prefix)
		if !found {
			continue
		}
		if version != "" && !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		return version, nil
	}

	return "", ErrOperatorNotInstalled
}

// HasSubscription reports whether a named Subscription exists in a namespace.
// The namespace is explicit because OLM subscriptions are scoped.
func HasSubscription(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	name string,
) (bool, error) {
	_, err := GetSubscription(ctx, reader, namespace, name)
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, err
	}
}

// GetSubscription returns a namespaced OLM Subscription as unstructured data.
func GetSubscription(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	name string,
) (*unstructured.Unstructured, error) {
	if namespace == "" {
		return nil, ErrNamespaceRequired
	}

	subscription := new(unstructured.Unstructured)
	subscription.SetGroupVersionKind(gvk.Subscription)

	if err := reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, subscription); err != nil {
		return nil, err
	}

	return subscription, nil
}

// HasCatalogSource reports whether a namespaced CatalogSource exists.
// NotFound means false; missing OLM APIs and other failures are returned.
func HasCatalogSource(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	name string,
) (bool, error) {
	_, err := GetCatalogSource(ctx, reader, namespace, name)
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, err
	}
}

// GetCatalogSource returns a namespaced OLM CatalogSource as unstructured data.
func GetCatalogSource(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	name string,
) (*unstructured.Unstructured, error) {
	if namespace == "" {
		return nil, ErrNamespaceRequired
	}

	catalogSource := new(unstructured.Unstructured)
	catalogSource.SetGroupVersionKind(gvk.CatalogSource)

	if err := reader.Get(
		ctx,
		client.ObjectKey{Namespace: namespace, Name: name},
		catalogSource,
	); err != nil {
		return nil, err
	}

	return catalogSource, nil
}
