package resources

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Decode reads multi-document YAML or JSON into unstructured resources. Empty
// documents and documents without a kind are ignored.
func Decode(content []byte) ([]*unstructured.Unstructured, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 4096)
	objects := make([]*unstructured.Unstructured, 0)

	for {
		var raw map[string]any

		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return objects, nil
		}

		if err != nil {
			return nil, fmt.Errorf("decode resource: %w", err)
		}

		if len(raw) == 0 {
			continue
		}

		if kind, _ := raw["kind"].(string); kind == "" {
			continue
		}

		objects = append(objects, &unstructured.Unstructured{Object: raw})
	}
}

// ToUnstructured converts a Kubernetes object to an unstructured object.
func ToUnstructured(object client.Object) (*unstructured.Unstructured, error) {
	if u, ok := object.(*unstructured.Unstructured); ok {
		return u.DeepCopy(), nil
	}

	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return nil, fmt.Errorf("convert %T to unstructured: %w", object, err)
	}

	return &unstructured.Unstructured{Object: data}, nil
}

// DecodeJSON decodes one JSON object into an unstructured resource.
func DecodeJSON(content []byte) (*unstructured.Unstructured, error) {
	var object map[string]any

	err := json.Unmarshal(content, &object)
	if err != nil {
		return nil, fmt.Errorf("decode JSON resource: %w", err)
	}

	kind, _ := object["kind"].(string)
	if kind == "" {
		return nil, ErrResourceKindRequired
	}

	return &unstructured.Unstructured{Object: object}, nil
}
