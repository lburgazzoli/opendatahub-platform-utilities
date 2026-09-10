//nolint:exhaustruct_v5 // Sparse literals keep focused watcher fixtures readable.
package dynamicwatcher

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	kubeMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func TestSyncConfiguredEvaluatesConditionsAndRegistersOnce(t *testing.T) {
	t.Parallel()

	controllerInstance := &mockController{}
	controllerInstance.On("Watch", mock.Anything).Return(nil).Once()
	watchGVK := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Dependency"}
	watcher := &Watcher{
		controller: controllerInstance,
		cache:      nil,
		registered: sets.New[watchKey](),
		registrations: []Registration{{
			Object:       resources.GvkToUnstructured(watchGVK),
			EventHandler: handler.RequestFromObject(),
			Predicates:   []predicate.Predicate{predicate.Funcs{}},
			DynamicPredicates: []DynamicPredicate{
				func(_ context.Context, _ *pipeline.Request) (bool, error) {
					return true, nil
				},
			},
		}},
	}
	request := &pipeline.Request{}

	g := NewWithT(t)
	g.Expect(watcher.SyncConfigured(t.Context(), request)).Should(Succeed())
	g.Expect(watcher.SyncConfigured(t.Context(), request)).Should(Succeed())
	g.Expect(watcher.registered.Has(watchKey{gvk: watchGVK, route: routeConfigured})).To(BeTrue())
	g.Expect(controllerInstance.AssertExpectations(t)).To(BeTrue())
}

func TestSyncConfiguredDeduplicatesSameGVKInputs(t *testing.T) {
	t.Parallel()

	controllerInstance := &mockController{}
	controllerInstance.On("Watch", mock.Anything).Return(nil).Once()
	watchGVK := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Dependency"}
	watcher := &Watcher{
		controller: controllerInstance,
		registered: sets.New[watchKey](),
		registrations: []Registration{
			{
				Object:       resources.GvkToUnstructured(watchGVK),
				EventHandler: handler.RequestFromObject(),
				DynamicPredicates: []DynamicPredicate{
					func(_ context.Context, _ *pipeline.Request) (bool, error) {
						return true, nil
					},
				},
			},
			{
				Object:       resources.GvkToUnstructured(watchGVK),
				EventHandler: handler.ToNamed("second"),
				DynamicPredicates: []DynamicPredicate{
					func(_ context.Context, _ *pipeline.Request) (bool, error) {
						return true, nil
					},
				},
			},
		},
	}

	g := NewWithT(t)
	request := &pipeline.Request{}
	g.Expect(watcher.SyncConfigured(t.Context(), request)).Should(Succeed())
	g.Expect(watcher.SyncConfigured(t.Context(), request)).Should(Succeed())

	g.Expect(watcher.registered.Has(watchKey{
		gvk:   watchGVK,
		route: routeConfigured,
	})).To(BeTrue())
	g.Expect(controllerInstance.AssertExpectations(t)).To(BeTrue())
}

func TestSyncConfiguredStopsOnConditionError(t *testing.T) {
	t.Parallel()

	watchGVK := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Dependency"}
	watcher := &Watcher{
		registered: sets.New[watchKey](),
		registrations: []Registration{{
			Object: resources.GvkToUnstructured(watchGVK),
			DynamicPredicates: []DynamicPredicate{
				func(_ context.Context, _ *pipeline.Request) (bool, error) {
					return false, errCondition
				},
			},
		}},
	}

	g := NewWithT(t)
	g.Expect(watcher.SyncConfigured(t.Context(), &pipeline.Request{})).Should(
		MatchError(ContainSubstring("condition failed")),
	)
}

func TestSyncUsesOwnedUnmanagedAndCRDRoutes(t *testing.T) {
	t.Parallel()

	controllerInstance := &mockController{}
	controllerInstance.On("Watch", mock.Anything).Return(nil).Times(3)
	primaryGVK := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Component"}
	childGVK := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Dependency"}
	mapper := kubeMeta.NewDefaultRESTMapper([]schema.GroupVersion{primaryGVK.GroupVersion()})
	mapper.Add(primaryGVK, kubeMeta.RESTScopeNamespace)
	mapper.Add(childGVK, kubeMeta.RESTScopeNamespace)

	owned := unstructured.Unstructured{Object: map[string]any{}}
	owned.SetGroupVersionKind(childGVK)
	owned.SetName("owned")
	owned.SetNamespace("default")

	unmanaged := unstructured.Unstructured{Object: map[string]any{}}
	unmanaged.SetGroupVersionKind(childGVK)
	unmanaged.SetName("unmanaged")
	unmanaged.SetNamespace("default")
	unmanaged.SetAnnotations(map[string]string{"opendatahub.io/managed": "false"})

	crd := unstructured.Unstructured{Object: map[string]any{}}
	crd.SetGroupVersionKind(gvk.CustomResourceDefinition)
	crd.SetName("dependencies.example.io")

	watcher := &Watcher{
		controller:     controllerInstance,
		mapper:         mapper,
		primaryGVK:     primaryGVK,
		primaryMapping: &kubeMeta.RESTMapping{Scope: kubeMeta.RESTScopeNamespace},
		registered:     sets.New[watchKey](),
	}
	request := &pipeline.Request{
		Instance: &consumer.Consumer{ObjectMeta: metav1.ObjectMeta{Name: "component"}},
		Resources: resources.New(resources.List{
			owned,
			unmanaged,
			crd,
		}),
	}

	g := NewWithT(t)
	g.Expect(watcher.Sync(t.Context(), request, nil)).Should(Succeed())
	g.Expect(watcher.registered.Has(watchKey{gvk: childGVK, route: routeOwned})).To(BeTrue())
	g.Expect(watcher.registered.Has(watchKey{gvk: childGVK, route: routeUnowned})).To(BeTrue())
	g.Expect(watcher.registered.Has(watchKey{
		gvk:   gvk.CustomResourceDefinition,
		route: routeCRD,
		name:  crd.GetName(),
	})).To(BeTrue())
	g.Expect(controllerInstance.AssertExpectations(t)).To(BeTrue())
}

var errCondition = errors.New("condition failed") // Test-only sentinel.
