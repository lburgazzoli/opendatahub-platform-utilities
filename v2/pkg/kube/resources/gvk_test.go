package resources_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGvkToObjects(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	gvk := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Component"}

	unstructuredObject := resources.GvkToUnstructured(gvk)
	partialObject := resources.GvkToPartial(gvk)

	g.Expect(unstructuredObject).To(BeAssignableToTypeOf(&unstructured.Unstructured{}))
	g.Expect(unstructuredObject.GroupVersionKind()).To(Equal(gvk))
	g.Expect(partialObject.GroupVersionKind()).To(Equal(gvk))
}
