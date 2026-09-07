package deletion

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func cloneTypes(types []client.Object) ([]client.Object, error) {
	if len(types) == 0 {
		return nil, nil
	}

	cloned := make([]client.Object, 0, len(types))
	for _, prototype := range types {
		if prototype == nil {
			return nil, ErrTypeInvalid
		}

		value := reflect.ValueOf(prototype)
		if value.Kind() == reflect.Pointer && value.IsNil() {
			return nil, ErrTypeInvalid
		}

		copyObject, ok := prototype.DeepCopyObject().(client.Object)
		if !ok || copyObject == nil {
			return nil, ErrTypeInvalid
		}

		cloned = append(cloned, copyObject)
	}

	return cloned, nil
}
