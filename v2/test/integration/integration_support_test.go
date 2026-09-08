//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/gc"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
)

const (
	testNamespace = "default"
	testTimeout   = 2 * time.Minute
	testPolling   = 500 * time.Millisecond

	componentGroup   = "integration.odh.io"
	componentVersion = "v1alpha1"
	componentKind    = "IntegrationComponent"
	componentPlural  = "integrationcomponents"
)

//nolint:gochecknoglobals // The integration fixture GVK is immutable.
var componentGVK = schema.GroupVersionKind{
	Group:   componentGroup,
	Version: componentVersion,
	Kind:    componentKind,
}

const helmChartYAML = `apiVersion: v2
name: kind-integration
description: Kind integration test chart
type: application
version: 0.1.0
appVersion: "1.0"
`

const helmTemplateYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.name }}-rendered
  namespace: {{ .Values.namespace }}
data:
  source: kind-integration
`

func setupController(t *testing.T, manager ctrlmanager.Manager) error {
	t.Helper()

	chartPath := writeChart(t)
	renderer, err := helm.NewEngine(
		helm.Source{Chart: chartPath, ReleaseName: "kind-integration"},
	)
	if err != nil {
		return fmt.Errorf("create Helm renderer: %w", err)
	}

	deployAction := deploy.New()
	gcAction := gc.New(gc.StaticDiscovery(gvk.ConfigMap))

	return reconciler.For(
		manager,
		new(consumer.Consumer),
		reconciler.WithControllerName("kind-integration"),
		reconciler.WithFieldOwner("kind-integration"),
	).
		Owns(&corev1.ConfigMap{}).
		WithActionFunc(func(ctx context.Context, request *pipeline.Request) error {
			component, err := reconciler.Instance[*consumer.Consumer](request)
			if err != nil {
				return err
			}

			rendered, err := renderer.Render(ctx, manifestengine.WithValues(map[string]any{
				"name":      component.GetName(),
				"namespace": component.GetNamespace(),
			}))
			if err != nil {
				return fmt.Errorf("render Helm chart: %w", err)
			}

			request.Resources.Set(rendered)
			return nil
		}, pipeline.WithName("render")).
		WithAction(deployAction).
		WithAfterAction(gcAction).
		WithCleanupActionFunc(deleteManagedConfigMap).
		Build()
}

func deleteManagedConfigMap(ctx context.Context, request *pipeline.Request) error {
	component, err := reconciler.Instance[*consumer.Consumer](request)
	if err != nil {
		return err
	}

	managed := resources.GvkToUnstructured(gvk.ConfigMap)
	managed.SetNamespace(component.GetNamespace())
	managed.SetName(managedConfigMapName(component))

	err = request.Client.Get(ctx, client.ObjectKeyFromObject(managed), managed)
	switch {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("lookup managed ConfigMap: %w", err)
	}

	err = request.Client.Delete(ctx, managed)
	switch {
	case err == nil:
		return nil
	case apierrors.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete managed ConfigMap: %w", err)
	}
}

func managedConfigMapName(component client.Object) string {
	return component.GetName() + "-rendered"
}

func staleConfigMap(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      "stale-managed-configmap",
	}}
}

func integrationScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	scheme.AddKnownTypes(componentGVK.GroupVersion(), new(consumer.Consumer), new(consumer.ConsumerList))
	metav1.AddToGroupVersion(scheme, componentGVK.GroupVersion())

	return scheme
}

func installComponentCRD(t *testing.T, kubeClient client.Client) {
	t.Helper()

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: componentPlural + "." + componentGroup},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: componentGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: componentPlural,
				Kind:   componentKind,
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    componentVersion,
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: new(true),
						},
					},
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
					},
				},
			},
		},
	}

	g := NewWithT(t)
	g.Expect(kubeClient.Create(t.Context(), crd)).To(Succeed())
	g.Eventually(func(g Gomega) {
		current := new(apiextensionsv1.CustomResourceDefinition)
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(crd), current)
		g.Expect(err).To(Succeed())
		g.Expect(current.Status.Conditions).To(ContainElement(Satisfy(
			func(condition apiextensionsv1.CustomResourceDefinitionCondition) bool {
				return condition.Type == apiextensionsv1.Established &&
					condition.Status == apiextensionsv1.ConditionTrue
			},
		)))
	}).WithContext(t.Context()).WithTimeout(testTimeout).WithPolling(testPolling).Should(Succeed())
}

func newRuntimeManager(kubeConfig *rest.Config, scheme *runtime.Scheme) (ctrlmanager.Manager, error) {
	return ctrl.NewManager(kubeConfig, ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: "0",
		Cache: cache.Options{
			ReaderFailOnMissingInformer: true,
			DefaultNamespaces: map[string]cache.Config{
				testNamespace: {},
			},
		},
		Client: client.Options{
			Cache: &client.CacheOptions{Unstructured: true},
		},
	})
}

func startManager(t *testing.T, manager ctrlmanager.Manager) {
	t.Helper()

	g := NewWithT(t)
	managerContext, cancelManager := context.WithCancel(t.Context())
	managerDone := make(chan error, 1)
	go func() {
		managerDone <- manager.Start(managerContext)
	}()

	managerFinished := false
	t.Cleanup(func() {
		cancelManager()
		if !managerFinished {
			g.Expect(<-managerDone).To(Succeed())
		}
	})

	startupContext, cancelStartup := context.WithTimeout(t.Context(), testTimeout)
	defer cancelStartup()

	cacheReady := make(chan bool, 1)
	go func() {
		cacheReady <- manager.GetCache().WaitForCacheSync(startupContext)
	}()

	select {
	case err := <-managerDone:
		managerFinished = true
		g.Expect(err).NotTo(HaveOccurred())
	case synced := <-cacheReady:
		g.Expect(synced).To(BeTrue())
	case <-startupContext.Done():
		g.Expect(startupContext.Err()).NotTo(HaveOccurred())
	}
}

func writeChart(t *testing.T) string {
	t.Helper()

	chartPath := t.TempDir() + "/chart"
	err := os.MkdirAll(chartPath+"/templates", 0o750)
	if err != nil {
		t.Fatalf("create chart directory: %v", err)
	}
	err = os.WriteFile(chartPath+"/Chart.yaml", []byte(helmChartYAML), 0o600)
	if err != nil {
		t.Fatalf("write Chart.yaml: %v", err)
	}
	err = os.WriteFile(chartPath+"/templates/configmap.yaml", []byte(helmTemplateYAML), 0o600)
	if err != nil {
		t.Fatalf("write Helm template: %v", err)
	}

	return chartPath
}

func runtimeUnavailable(err error) bool {
	message := strings.ToLower(err.Error())

	return strings.Contains(message, "failed to get docker info") ||
		strings.Contains(message, "cannot connect to the docker daemon") ||
		strings.Contains(message, "failed to get podman info") ||
		strings.Contains(message, "executable file not found")
}
