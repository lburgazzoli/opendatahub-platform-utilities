package deploy_test

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
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
				scheme := runtime.NewScheme()
				err := corev1.AddToScheme(scheme)
				if err != nil {
					b.Fatal(err)
				}
				owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
					Name: "owner", Namespace: "ns", UID: "owner-uid",
				}}
				owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
				objects := make(resources.List, count)
				for index := range count {
					object := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("desired-%d", index),
						Namespace: "ns",
					}}
					object.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
					objects[index] = object
				}
				kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
				options := []deploy.Option(nil)
			if cached {
				options = append(options, deploy.WithCache())
			}
			action := deploy.New(options...)

			if cached {
				_, err = action.Run(b.Context(), deploy.RunOptions{
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
					_, err := action.Run(b.Context(), deploy.RunOptions{
						Client:    kubernetesClient,
						Owner:     owner,
						Resources: collection,
					})
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
