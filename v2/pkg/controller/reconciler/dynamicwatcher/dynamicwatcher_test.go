package dynamicwatcher

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	kubeMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	platformhandler "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
)

func TestRegisterRecordsOnlySuccessfulWatches(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	controllerInstance := &mockController{}
	controllerInstance.On("Watch", mock.Anything).Return(errWatchRegistration).Once()
	controllerInstance.On("Watch", mock.Anything).Return(nil).Once()
	gvk := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Component"}
	watcher := &Watcher{
		controller: controllerInstance,
		registered: sets.New[schema.GroupVersionKind](),
	}

	g.Expect(watcher.register(gvk)).Should(MatchError(ContainSubstring("watch registration failed")))
	g.Expect(watcher.register(gvk)).Should(Succeed())
	g.Expect(watcher.register(gvk)).Should(Succeed())
	g.Expect(controllerInstance.AssertExpectations(t)).Should(BeTrue())
}

func TestRequestMapperPrefersControllerOwner(t *testing.T) {
	t.Parallel()

	controller := new(true)
	object := &unstructured.Unstructured{Object: map[string]any{}}
	object.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	object.SetNamespace("workload")
	object.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "example.io/v1",
			Kind:       "Component",
			Name:       "owner",
			Controller: controller,
		},
	})
	object.SetAnnotations(map[string]string{
		platformannotations.InstanceName: "annotated-owner",
	})

	watcher := &Watcher{
		primaryGVK:        schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Component"},
		primaryMapping:    &kubeMeta.RESTMapping{Scope: kubeMeta.RESTScopeNamespace},
		annotationHandler: platformhandler.EnqueueByOwnerAnnotation(),
	}

	g := NewWithT(t)
	requests := watcher.requestMapper(t.Context(), object)
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests[0].Name).To(Equal("owner"))
	g.Expect(requests[0].Namespace).To(Equal("workload"))
}

func TestRequestMapperUsesEmptyNamespaceForClusterOwner(t *testing.T) {
	t.Parallel()

	controller := new(true)
	object := &unstructured.Unstructured{Object: map[string]any{}}
	object.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	object.SetNamespace("workload")
	object.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "example.io/v1",
			Kind:       "Component",
			Name:       "owner",
			Controller: controller,
		},
	})

	watcher := &Watcher{
		primaryGVK:        schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Component"},
		primaryMapping:    &kubeMeta.RESTMapping{Scope: kubeMeta.RESTScopeRoot},
		annotationHandler: platformhandler.EnqueueByOwnerAnnotation(),
	}

	g := NewWithT(t)
	requests := watcher.requestMapper(t.Context(), object)
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests[0].Name).To(Equal("owner"))
	g.Expect(requests[0].Namespace).To(BeEmpty())
}

func TestCRDRequestMapperUsesAnnotations(t *testing.T) {
	t.Parallel()

	object := &unstructured.Unstructured{Object: map[string]any{}}
	object.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition",
	})
	object.SetAnnotations(map[string]string{
		platformannotations.InstanceName:      "owner",
		platformannotations.InstanceNamespace: "platform-system",
	})
	object.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "example.io/v1",
		Kind:       "Component",
		Name:       "ignored-owner-reference",
	}})

	watcher := &Watcher{
		primaryMapping:    &kubeMeta.RESTMapping{Scope: kubeMeta.RESTScopeRoot},
		annotationHandler: platformhandler.EnqueueByOwnerAnnotation(),
	}

	g := NewWithT(t)
	requests := watcher.annotationHandler(t.Context(), object)
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests[0].Name).To(Equal("owner"))
	g.Expect(requests[0].Namespace).To(Equal("platform-system"))
}
