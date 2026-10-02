package deploy

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

func BenchmarkRun(b *testing.B) {
	benchmarkRun(b)
}

func benchmarkRun(b *testing.B) {
	b.Helper()

	for _, count := range []int{1, 10, 100} {
		for _, cached := range []bool{false, true} {
			b.Run(fmt.Sprintf("resources-%d/cached-%t", count, cached), func(b *testing.B) {
				scheme := benchmarkScheme(b)
				owner := benchmarkOwner()
				objects := benchmarkObjects(count)

				kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
				options := []Option(nil)

				if !cached {
					options = append(options, WithCache(false))
				}

				action := New(options...)

				if cached {
					_, err := action.Run(b.Context(), RunOptions{
						Client:    kubernetesClient,
						Owner:     owner,
						Resources: resources.New(objects),
					})
					if err != nil {
						b.Fatal(err)
					}
				}

				b.ReportAllocs()

				b.ResetTimer()
				for b.Loop() {
					collection := resources.New(objects)
					_, err := action.Run(b.Context(), RunOptions{
						Client:    kubernetesClient,
						Owner:     owner,
						Resources: collection,
					})
					if err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(count), "resources/op")
			})
		}
	}
}

func BenchmarkPrepare(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("resources-%d", count), func(b *testing.B) {
			scheme := benchmarkScheme(b)
			kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			owner := benchmarkOwner()
			objects := benchmarkObjects(count)
			action := New()

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				collection := resources.New(objects)
				_, err := action.prepare(RunOptions{
					Client:    kubernetesClient,
					Owner:     owner,
					Resources: collection,
				})
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(count), "resources/op")
		})
	}
}

func BenchmarkCacheHas(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("resources-%d", count), func(b *testing.B) {
			cache := NewCache(&CacheOptions{})
			objects := benchmarkObjects(count)
			desired := make([]*unstructured.Unstructured, 0, count)
			deployed := make([]*unstructured.Unstructured, 0, count)

			for index := range objects {
				desiredObject := &objects[index]
				deployedObject := desiredObject.DeepCopy()
				desired = append(desired, desiredObject)
				deployed = append(deployed, deployedObject)

				if err := cache.Add(deployedObject, desiredObject); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				for index := range desired {
					cached, err := cache.Has(deployed[index], desired[index])
					if err != nil {
						b.Fatal(err)
					}
					if !cached {
						b.Fatal("expected cache hit")
					}
				}
			}
			b.ReportMetric(float64(count), "resources/op")
		})
	}
}

func BenchmarkApply(b *testing.B) {
	for _, objectType := range []string{"typed", "unstructured"} {
		b.Run(objectType, func(b *testing.B) {
			scheme := benchmarkScheme(b)
			kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			var desired client.Object
			switch objectType {
			case "typed":
				desired = benchmarkTypedObject()
			case "unstructured":
				objects := benchmarkObjects(1)
				desired = &objects[0]
			}

			if err := resources.Apply(
				b.Context(),
				kubernetesClient,
				desired,
				client.FieldOwner("benchmark"),
				client.ForceOwnership,
			); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if err := resources.Apply(
					b.Context(),
					kubernetesClient,
					desired,
					client.FieldOwner("benchmark"),
					client.ForceOwnership,
				); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRunNoOpClient(b *testing.B) {
	for _, count := range []int{1, 10, 100} {
		for _, cached := range []bool{false, true} {
			b.Run(fmt.Sprintf("resources-%d/cached-%t", count, cached), func(b *testing.B) {
				scheme := benchmarkScheme(b)
				owner := benchmarkOwner()
				objects := benchmarkObjects(count)
				kubernetesClient := newBenchmarkClient(b, scheme, objects, cached)

				var action *Action
				if cached {
					action = New()
				} else {
					action = New(WithCache(false))
				}

				if cached {
					_, err := action.Run(b.Context(), RunOptions{
						Client:    kubernetesClient,
						Owner:     owner,
						Resources: resources.New(objects),
					})
					if err != nil {
						b.Fatal(err)
					}
				}

				b.ReportAllocs()
				b.ResetTimer()

				for b.Loop() {
					_, err := action.Run(b.Context(), RunOptions{
						Client:    kubernetesClient,
						Owner:     owner,
						Resources: resources.New(objects),
					})
					if err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(count), "resources/op")
			})
		}
	}
}

type benchmarkClient struct {
	client.Client

	current map[client.ObjectKey]*unstructured.Unstructured
}

func (c *benchmarkClient) Get(
	_ context.Context,
	key client.ObjectKey,
	object client.Object,
	_ ...client.GetOption,
) error {
	current, found := c.current[key]
	if !found {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, key.Name)
	}

	desired, ok := object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("benchmark client received %T", object)
	}

	*desired = *current.DeepCopy()

	return nil
}

func (c *benchmarkClient) Apply(
	_ context.Context,
	_ runtime.ApplyConfiguration,
	_ ...client.ApplyOption,
) error {
	return nil
}

func benchmarkScheme(b *testing.B) *runtime.Scheme {
	b.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		b.Fatal(err)
	}

	return scheme
}

func newBenchmarkClient(
	b *testing.B,
	scheme *runtime.Scheme,
	objects resources.List,
	withCurrent bool,
) client.Client {
	b.Helper()

	benchmark := &benchmarkClient{
		Client:  fake.NewClientBuilder().WithScheme(scheme).Build(),
		current: make(map[client.ObjectKey]*unstructured.Unstructured),
	}
	if !withCurrent {
		return benchmark
	}

	for _, object := range objects {
		desired := object.DeepCopy()
		benchmark.current[client.ObjectKeyFromObject(desired)] = desired
	}

	return benchmark
}

func benchmarkOwner() *corev1.ConfigMap {
	owner := &corev1.ConfigMap{}
	owner.SetName("owner")
	owner.SetNamespace("ns")
	owner.SetUID("owner-uid")
	owner.SetGroupVersionKind(gvk.ConfigMap)

	return owner
}

func benchmarkObjects(count int) resources.List {
	objects := make(resources.List, count)
	for index := range count {
		object := unstructured.Unstructured{Object: map[string]any{}}
		object.SetName(fmt.Sprintf("desired-%d", index))
		object.SetNamespace("ns")
		object.SetGroupVersionKind(gvk.ConfigMap)
		objects[index] = object
	}

	return objects
}

func benchmarkTypedObject() *corev1.ConfigMap {
	object := &corev1.ConfigMap{}
	object.SetName("desired-0")
	object.SetNamespace("ns")
	object.SetGroupVersionKind(gvk.ConfigMap)

	return object
}
