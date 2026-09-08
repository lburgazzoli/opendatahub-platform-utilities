package handler_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
)

func TestMappings(t *testing.T) {
	t.Parallel()

	object := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
		Namespace: "source",
		Name:      "child",
		Labels:    map[string]string{"owner": "label-owner"},
		Annotations: map[string]string{
			"owner.annotation":                    "annotation-owner",
			platformannotations.InstanceName:      "instance",
			platformannotations.InstanceNamespace: "instance-ns",
		},
	}}

	g := NewWithT(t)
	for name, eventHandler := range map[string]func() []reconcile.Request{
		"request": func() []reconcile.Request {
			return queuedRequests(t, t.Context(), handler.RequestFromObject(), object)
		},
		"label": func() []reconcile.Request {
			return queuedRequests(t, t.Context(), handler.LabelToName("owner"), object)
		},
		"annotation": func() []reconcile.Request {
			return queuedRequests(t, t.Context(), handler.AnnotationToName("owner.annotation"), object)
		},
		"owner annotation": func() []reconcile.Request {
			return handler.EnqueueByOwnerAnnotation()(t.Context(), object)
		},
	} {
		g.Expect(eventHandler()).To(HaveLen(1), name)
	}

	g.Expect(queuedRequests(t, t.Context(), handler.RequestFromObject(), object)[0].NamespacedName).To(
		Equal(types.NamespacedName{Namespace: "source", Name: "child"}),
	)
	g.Expect(queuedRequests(t, t.Context(), handler.LabelToName("owner"), object)[0].NamespacedName).To(
		Equal(types.NamespacedName{Namespace: "source", Name: "label-owner"}),
	)
	g.Expect(queuedRequests(t, t.Context(), handler.AnnotationToName("owner.annotation"), object)[0].NamespacedName).To(
		Equal(types.NamespacedName{Namespace: "source", Name: "annotation-owner"}),
	)
	g.Expect(handler.EnqueueByOwnerAnnotation()(t.Context(), object)[0].NamespacedName).To(
		Equal(types.NamespacedName{Namespace: "instance-ns", Name: "instance"}),
	)
}

func TestMappingsIgnoreMissingValues(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	object := &metav1.PartialObjectMetadata{}
	g.Expect(queuedRequests(t, t.Context(), handler.LabelToName("owner"), object)).To(BeEmpty())
	g.Expect(queuedRequests(t, t.Context(), handler.AnnotationToName("owner"), object)).To(BeEmpty())
	g.Expect(handler.EnqueueByOwnerAnnotation()(t.Context(), object)).To(BeEmpty())
	g.Expect(queuedRequests(t, t.Context(), handler.ToNamed(""), object)).To(BeEmpty())
}

func queuedRequests(
	t *testing.T,
	ctx context.Context,
	eventHandler interface {
		Create(
			ctx context.Context,
			value event.TypedCreateEvent[client.Object],
			queue workqueue.TypedRateLimitingInterface[reconcile.Request],
		)
	},
	object client.Object,
) []reconcile.Request {
	t.Helper()
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	eventHandler.Create(ctx, event.TypedCreateEvent[client.Object]{Object: object}, queue)
	defer queue.ShutDown()

	if queue.Len() == 0 {
		return nil
	}

	request, shutdown := queue.Get()
	if shutdown {
		return nil
	}
	queue.Done(request)

	return []reconcile.Request{request}
}
