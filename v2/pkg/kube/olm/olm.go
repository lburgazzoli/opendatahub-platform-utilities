package olm

import (
	"context"
	"errors"
	"strings"

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

	list := new(unstructured.UnstructuredList)
	list.SetGroupVersionKind(gvk.OperatorCondition)

	if err := reader.List(ctx, list, client.InNamespace(namespace)); err != nil {
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

// SubscriptionExists reports whether a named Subscription exists in a
// namespace. The namespace is explicit because OLM subscriptions are scoped.
func SubscriptionExists(ctx context.Context, reader client.Reader, namespace string, name string) (bool, error) {
	if namespace == "" {
		return false, ErrNamespaceRequired
	}

	list := new(unstructured.UnstructuredList)
	list.SetGroupVersionKind(gvk.Subscription)

	if err := reader.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return false, err
	}

	for _, item := range list.Items {
		if item.GetName() == name {
			return true, nil
		}
	}

	return false, nil
}

// GetSubscription returns a namespaced OLM Subscription as unstructured data.
func GetSubscription(ctx context.Context, reader client.Reader, namespace string, name string) (*unstructured.Unstructured, error) {
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

// CatalogSourceExists reports whether a namespaced CatalogSource exists.
// NotFound means false; missing OLM APIs and other failures are returned.
func CatalogSourceExists(ctx context.Context, reader client.Reader, namespace string, name string) (bool, error) {
	if namespace == "" {
		return false, ErrNamespaceRequired
	}

	catalogSource := new(unstructured.Unstructured)
	catalogSource.SetGroupVersionKind(gvk.CatalogSource)

	err := reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, catalogSource)
	if err == nil {
		return true, nil
	}

	if client.IgnoreNotFound(err) == nil {
		return false, nil
	}

	return false, err
}
