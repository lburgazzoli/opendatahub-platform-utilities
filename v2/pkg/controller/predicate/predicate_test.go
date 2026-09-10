package predicate_test

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
)

func TestGenerationChanged(t *testing.T) {
	t.Parallel()

	oldObject := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
	newObject := oldObject.DeepCopy()
	newObject.Generation = 2

	g := NewWithT(t)
	g.Expect(predicate.GenerationChanged().Update(event.UpdateEvent{
		ObjectOld: oldObject,
		ObjectNew: newObject,
	})).To(BeTrue())
}

func TestFuncsDefaultToFalse(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	var funcs predicate.Funcs

	g.Expect(funcs.Create(event.CreateEvent{})).To(BeFalse())
	g.Expect(funcs.Delete(event.DeleteEvent{})).To(BeFalse())
	g.Expect(funcs.Generic(event.GenericEvent{})).To(BeFalse())
	g.Expect(funcs.Update(event.UpdateEvent{})).To(BeFalse())
}

func TestHasAnnotation(t *testing.T) {
	t.Parallel()

	object := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{"example.io/managed": "true"},
	}}

	g := NewWithT(t)
	g.Expect(predicate.HasAnnotation("example.io/managed").Create(event.CreateEvent{Object: object})).To(BeTrue())
	g.Expect(
		predicate.HasAnnotation("example.io/managed", "false").Create(event.CreateEvent{Object: object}),
	).To(BeFalse())
}

func TestPartOfWithLabelUsesConfiguredLabel(t *testing.T) {
	t.Parallel()

	oldObject := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
		Generation: 1,
		Labels:     map[string]string{"app.kubernetes.io/part-of": "component"},
	}}
	newObject := oldObject.DeepCopy()
	newObject.Generation = 2

	g := NewWithT(t)
	g.Expect(predicate.PartOfWithLabel("app.kubernetes.io/part-of", "component").Update(event.UpdateEvent{
		ObjectOld: oldObject,
		ObjectNew: newObject,
	})).To(BeTrue())
	g.Expect(predicate.PartOf("component").Update(event.UpdateEvent{
		ObjectOld: oldObject,
		ObjectNew: newObject,
	})).To(BeFalse())
}

func TestDependentIgnoresStatusOnlyUpdates(t *testing.T) {
	t.Parallel()

	oldObject := unstructuredObject(map[string]any{
		"spec":   map[string]any{"replicas": int64(1)},
		"status": map[string]any{"ready": int64(0)},
	})
	newObject := oldObject.DeepCopy()
	newObject.Object["status"] = map[string]any{"ready": int64(1)}
	oldObject.SetResourceVersion("1")
	newObject.SetResourceVersion("2")

	g := NewWithT(t)
	g.Expect(predicate.Dependent(predicate.DependentOptions{
		WatchUpdate: true,
	}).Update(event.UpdateEvent{
		ObjectOld: oldObject,
		ObjectNew: newObject,
	})).To(BeFalse())
}

func TestHashChangedIgnoresServerMetadata(t *testing.T) {
	t.Parallel()

	oldObject := unstructuredObject(map[string]any{"spec": map[string]any{"enabled": true}})
	newObject := oldObject.DeepCopy()
	oldObject.SetAnnotations(map[string]string{
		corev1.LastAppliedConfigAnnotation: "old",
	})
	newObject.SetAnnotations(map[string]string{
		corev1.LastAppliedConfigAnnotation: "new",
	})
	newObject.SetResourceVersion("2")
	newObject.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "server"}})

	g := NewWithT(t)
	g.Expect(predicate.HashChanged().Update(event.UpdateEvent{
		ObjectOld: oldObject,
		ObjectNew: newObject,
	})).To(BeFalse())
}

func TestDeploymentStatusChanged(t *testing.T) {
	t.Parallel()

	oldObject := unstructuredObject(map[string]any{
		"status": map[string]any{"replicas": int64(1), "readyReplicas": int64(0)},
	})
	newObject := oldObject.DeepCopy()
	newObject.Object["status"] = map[string]any{"replicas": int64(1), "readyReplicas": int64(1)}

	g := NewWithT(t)
	g.Expect(predicate.DeploymentStatusChanged().Update(event.UpdateEvent{
		ObjectOld: oldObject,
		ObjectNew: newObject,
	})).To(BeTrue())
}

func unstructuredObject(fields map[string]any) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: fields}
	object.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	return object
}
