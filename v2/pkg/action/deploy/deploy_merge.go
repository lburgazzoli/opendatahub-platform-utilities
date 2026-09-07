package deploy

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrFieldNotSlice = errors.New("field is not a slice")
)

// MergeDeployments preserves live replicas, container resources, and probes
// that are omitted from the desired Deployment.
func MergeDeployments(
	existing *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error {
	err := mergeDeploymentResources(existing, desired)
	if err != nil {
		return err
	}
	err = mergeDeploymentProbes(existing, desired)
	if err != nil {
		return err
	}
	value, found, err := unstructured.NestedFieldNoCopy(existing.Object, "spec", "replicas")
	if err != nil {
		return err
	}
	if !found {
		unstructured.RemoveNestedField(desired.Object, "spec", "replicas")
		return nil
	}
	return unstructured.SetNestedField(desired.Object, runtime.DeepCopyJSONValue(value), "spec", "replicas")
}

// MergeObservabilityResources preserves live spec.resources.
func MergeObservabilityResources(
	existing *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error {
	value, found, err := unstructured.NestedFieldNoCopy(existing.Object, "spec", "resources")
	if err != nil {
		return err
	}
	if found && value != nil {
		return unstructured.SetNestedField(desired.Object, runtime.DeepCopyJSONValue(value), "spec", "resources")
	}
	return nil
}

//nolint:cyclop // unstructured container merge needs shape checks per field.
func mergeDeploymentResources(
	existing *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error {
	live, err := containers(existing)
	if err != nil {
		return err
	}
	wanted, err := containers(desired)
	if err != nil {
		return err
	}
	resourcesByName := make(map[string]any, len(live))
	for _, value := range live {
		container, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name, ok := container["name"].(string)
		if !ok {
			continue
		}
		resourcesByName[name] = container["resources"]
	}
	for _, value := range wanted {
		container, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name, ok := container["name"].(string)
		if !ok {
			continue
		}
		liveResources, found := resourcesByName[name]
		if !found || liveResources == nil {
			delete(container, "resources")
			continue
		}
		container["resources"] = runtime.DeepCopyJSONValue(liveResources)
	}
	return nil
}

//nolint:cyclop // each probe field has independent preservation semantics.
func mergeDeploymentProbes(
	existing *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error {
	live, err := containers(existing)
	if err != nil {
		return err
	}
	wanted, err := containers(desired)
	if err != nil {
		return err
	}
	probes := make(map[string]map[string]any, len(live))
	for _, value := range live {
		container, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name, ok := container["name"].(string)
		if !ok {
			continue
		}
		probes[name] = map[string]any{}
		for _, field := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
			if probe, found := container[field]; found {
				probes[name][field] = probe
			}
		}
	}
	for _, value := range wanted {
		container, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name, ok := container["name"].(string)
		if !ok {
			continue
		}
		for _, field := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
			if _, found := container[field]; found {
				continue
			}
			if probe, found := probes[name][field]; found {
				container[field] = runtime.DeepCopyJSONValue(probe)
			}
		}
	}
	return nil
}

func containers(object *unstructured.Unstructured) ([]any, error) {
	value, found, err := unstructured.NestedFieldNoCopy(object.Object, "spec", "template", "spec", "containers")
	if err != nil {
		return nil, err
	}
	if !found || value == nil {
		return nil, nil
	}
	result, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("containers: %w", ErrFieldNotSlice)
	}
	return result, nil
}

func applyAggregatedClusterRoleCustomizer(
	_ context.Context,
	_ client.Client,
	desired *unstructured.Unstructured,
	existing *unstructured.Unstructured,
) error {
	if existing == nil {
		return nil
	}

	_, found, err := unstructured.NestedFieldNoCopy(desired.Object, "aggregationRule")
	if err != nil {
		return err
	} else if found {
		unstructured.RemoveNestedField(desired.Object, "rules")
	}

	return nil
}

func applyDeploymentCustomizer(
	_ context.Context,
	_ client.Client,
	desired *unstructured.Unstructured,
	existing *unstructured.Unstructured,
) error {
	if existing == nil {
		return nil
	}

	return MergeDeployments(existing, desired)
}

func applyObservabilityCustomizer(
	_ context.Context,
	_ client.Client,
	desired *unstructured.Unstructured,
	existing *unstructured.Unstructured,
) error {
	if existing == nil {
		return nil
	}

	return MergeObservabilityResources(existing, desired)
}
