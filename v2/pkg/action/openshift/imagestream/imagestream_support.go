package imagestream

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func listImageStreams(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	selectorLabels map[string]string,
) (*unstructured.UnstructuredList, bool, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(imageStreamGVK.GroupVersion().WithKind("ImageStreamList"))

	err := reader.List(
		ctx,
		list,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(selectorLabels)},
	)
	switch {
	case err == nil:
		return list, false, nil
	case meta.IsNoMatchError(err):
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("list ImageStreams: %w", err)
	}
}

func failedImageTags(list *unstructured.UnstructuredList) (int, []string, error) {
	failedCount := 0
	reported := make([]string, 0, maxFailedTags)

	for _, stream := range list.Items {
		tags, found, err := unstructured.NestedSlice(stream.Object, "status", "tags")
		switch {
		case err != nil:
			return 0, nil, fmt.Errorf("inspect ImageStream %s status.tags: %w", stream.GetName(), err)
		case !found:
			continue
		}

		for index, value := range tags {
			tag, ok := value.(map[string]any)
			if !ok {
				return 0, nil, fmt.Errorf(
					"%w: inspect ImageStream %s status.tags[%d]: expected object",
					ErrMalformedStatus,
					stream.GetName(),
					index,
				)
			}

			includeMessage := len(reported) < maxFailedTags
			failedTag, message, err := failedImageTag(stream.GetName(), tag, includeMessage)
			if err != nil {
				return 0, nil, err
			}
			if !failedTag {
				continue
			}

			failedCount++
			if includeMessage {
				reported = append(reported, message)
			}
		}
	}

	return failedCount, reported, nil
}

func healthyObservation(conditionType string) Observation {
	return Observation{Condition: api.Condition{
		Type:   conditionType,
		Status: metav1.ConditionTrue,
		Reason: "ImageStreamsReady",
	}}
}

func failedImageTag(name string, tag map[string]any, includeMessage bool) (bool, string, error) {
	hasItems, tagName, conditions, err := parseImageTag(name, tag)
	if err != nil {
		return false, "", err
	}

	for index, value := range conditions {
		conditionType, status, message, err := parseImageTagCondition(name, index, value)
		if err != nil {
			return false, "", err
		}
		if conditionType != "ImportSuccess" || status != "False" {
			continue
		}
		if hasItems {
			return false, "", nil
		}
		if !includeMessage {
			return true, "", nil
		}

		if len(message) > maxConditionMessageLen {
			message = message[:maxConditionMessageLen] + "..."
		}

		return true, fmt.Sprintf("%s:%s (%s)", name, tagName, message), nil
	}

	return false, "", nil
}

func parseImageTag(name string, tag map[string]any) (bool, string, []any, error) {
	items, _, err := unstructured.NestedSlice(tag, "items")
	if err != nil {
		return false, "", nil, fmt.Errorf("inspect ImageStream %s tag items: %w", name, err)
	}

	tagName, _, err := unstructured.NestedString(tag, "tag")
	if err != nil {
		return false, "", nil, fmt.Errorf("inspect ImageStream %s tag name: %w", name, err)
	}

	conditions, _, err := unstructured.NestedSlice(tag, "conditions")
	if err != nil {
		return false, "", nil, fmt.Errorf("inspect ImageStream %s tag conditions: %w", name, err)
	}

	return len(items) > 0, tagName, conditions, nil
}

func parseImageTagCondition(
	name string,
	index int,
	value any,
) (string, string, string, error) {
	current, ok := value.(map[string]any)
	if !ok {
		return "", "", "", fmt.Errorf(
			"%w: inspect ImageStream %s tag conditions[%d]: expected object",
			ErrMalformedStatus,
			name,
			index,
		)
	}

	conditionType, _, err := unstructured.NestedString(current, "type")
	if err != nil {
		return "", "", "", fmt.Errorf("inspect ImageStream %s tag condition type: %w", name, err)
	}

	status, _, err := unstructured.NestedString(current, "status")
	if err != nil {
		return "", "", "", fmt.Errorf("inspect ImageStream %s tag condition status: %w", name, err)
	}
	if conditionType != "ImportSuccess" || status != "False" {
		return conditionType, status, "", nil
	}

	message, _, err := unstructured.NestedString(current, "message")
	if err != nil {
		return "", "", "", fmt.Errorf("inspect ImageStream %s tag condition message: %w", name, err)
	}

	return conditionType, status, message, nil
}
