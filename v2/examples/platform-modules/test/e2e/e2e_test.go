//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onsi/gomega"
	kindtest "github.com/opendatahub-io/odh-platform-utilities/testkit/kind"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	aigatewayv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/aigateway/v1alpha1"
	kservev1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/kserve/v1alpha1"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/module"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/platform"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/internal/controller/serving"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var configureLogger sync.Once //nolint:gochecknoglobals // Controller-runtime logging is process-global.

func TestPlatformAndServingOnKind(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	image := os.Getenv("IMG")
	if image == "" {
		t.Skip("IMG is required for the module controller Deployment")
	}

	root := exampleRoot(t)
	registry, err := modules.Load(filepath.Join(root, "config", "modules"))
	g.Expect(err).To(gomega.Succeed())

	cluster, kubeClient, scheme := startCluster(t)
	assertModuleCRDsAbsent(t, kubeClient, registry)

	platformManager := newManager(t, cluster.RESTConfig(), scheme)
	g.Expect(platform.Setup(platformManager, registry, image, "default")).To(gomega.Succeed())
	startManager(t, platformManager)

	servingManager := newManager(t, cluster.RESTConfig(), scheme)
	g.Expect(serving.Setup(servingManager, registry)).To(gomega.Succeed())
	startManager(t, servingManager)

	serving := createServing(t, kubeClient)

	waitForModuleControllers(t, kubeClient, registry, image)

	waitForServingStatus(t, kubeClient, true)

	g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: serving.Name}, serving)).To(gomega.Succeed())
	serving.Spec.Kserve.ManagementState = "Removed"
	g.Expect(kubeClient.Update(t.Context(), serving)).To(gomega.Succeed())

	waitForKserveRemoval(t, kubeClient)
	waitForServingStatus(t, kubeClient, false)
}

func TestPlatformAndModuleOnKind(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	image := os.Getenv("IMG")
	if image == "" {
		t.Skip("IMG is required for the module controller Deployment")
	}

	root := exampleRoot(t)
	registry, err := modules.Load(filepath.Join(root, "config", "modules"))
	g.Expect(err).To(gomega.Succeed())
	cluster, kubeClient, scheme := startCluster(t)
	assertModuleCRDsAbsent(t, kubeClient, registry)

	platformManager := newManager(t, cluster.RESTConfig(), scheme)
	g.Expect(platform.Setup(platformManager, registry, image, "default")).To(gomega.Succeed())
	startManager(t, platformManager)

	instance := v1alpha1.NewPlatform()
	instance.Name = v1alpha1.PlatformName
	instance.Spec.Modules = []string{"kserve"}
	g.Expect(kubeClient.Create(t.Context(), instance)).To(gomega.Succeed())

	waitForModuleControllers(t, kubeClient, registry, image, "kserve")

	kserve := new(kservev1alpha1.Kserve)
	kserve.Name = v1alpha1.InstanceName
	kserve.Spec.ManagementState = "Managed"
	g.Eventually(func() error {
		return kubeClient.Create(t.Context(), kserve)
	}).WithContext(t.Context()).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())

	g.Eventually(func(g gomega.Gomega) {
		current := new(kservev1alpha1.Kserve)
		g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: kserve.Name}, current)).To(gomega.Succeed())
		g.Expect(condition.IsTrue(current.GetStatus(), string(platformapi.ConditionTypeReady))).To(gomega.BeTrue())
		g.Expect(current.Status.ObservedGeneration).To(gomega.Equal(current.Generation))
	}).WithContext(t.Context()).WithTimeout(3 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())

	g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: kserve.Name}, kserve)).To(gomega.Succeed())
	kserve.Annotations = map[string]string{module.SimulationAnnotation: "example dependency is unavailable"}
	g.Expect(kubeClient.Update(t.Context(), kserve)).To(gomega.Succeed())
	waitForModuleHealth(t, kubeClient, false, "example dependency is unavailable")

	g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: kserve.Name}, kserve)).To(gomega.Succeed())
	delete(kserve.Annotations, module.SimulationAnnotation)
	g.Expect(kubeClient.Update(t.Context(), kserve)).To(gomega.Succeed())
	waitForModuleHealth(t, kubeClient, true, "")
}

func waitForModuleHealth(t *testing.T, kubeClient client.Client, healthy bool, message string) {
	t.Helper()
	g := gomega.NewWithT(t)
	g.Eventually(func(g gomega.Gomega) {
		current := new(kservev1alpha1.Kserve)
		key := types.NamespacedName{Name: v1alpha1.InstanceName}
		g.Expect(kubeClient.Get(t.Context(), key, current)).To(gomega.Succeed())
		g.Expect(condition.IsTrue(current.GetStatus(), string(platformapi.ConditionTypeReady))).To(gomega.Equal(healthy))
		g.Expect(condition.IsTrue(current.GetStatus(), module.ConditionModuleConfigured)).To(gomega.BeTrue())
		g.Expect(condition.IsTrue(current.GetStatus(), module.ConditionSimulationActive)).To(gomega.Equal(!healthy))
		health := condition.Find(current.GetStatus(), string(platformapi.ConditionTypeProvisioningSucceeded))
		g.Expect(health).NotTo(gomega.BeNil())
		if healthy {
			g.Expect(health.Message).To(gomega.BeEmpty())
		} else {
			g.Expect(health.Message).To(gomega.ContainSubstring(message))
		}
	}).WithContext(t.Context()).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
}

func startCluster(t *testing.T) (*kindtest.Cluster, client.Client, *k8sruntime.Scheme) {
	t.Helper()
	configureLogger.Do(func() {
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	})
	g := gomega.NewWithT(t)
	logsDir := t.TempDir()
	clusterName := fmt.Sprintf("odh-platform-example-%d-%d", os.Getpid(), time.Now().UnixNano())
	engineOptions := []kindtest.Option{kindtest.WithName(clusterName), kindtest.WithLogsDir(logsDir)}
	if os.Getenv("KIND_PROVIDER") == "podman" {
		engineOptions = append(engineOptions, kindtest.WithPodman())
	}
	engine := kindtest.New(engineOptions...)
	cluster, err := engine.Start(t.Context())
	if err != nil {
		if runtimeUnavailable(err) {
			t.Skipf("container runtime is unavailable: %v", err)
		}
		g.Expect(err).To(gomega.Succeed())
	}
	t.Cleanup(func() {
		if t.Failed() {
			g.Expect(engine.CollectLogs(filepath.Join(logsDir, clusterName))).To(gomega.Succeed())
		}
		g.Expect(engine.Close(context.WithoutCancel(t.Context()))).To(gomega.Succeed())
	})

	scheme := k8sruntime.NewScheme()
	g.Expect(clientgoscheme.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(v1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(kservev1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(aigatewayv1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
	kubeClient, err := cluster.Client(scheme)
	g.Expect(err).To(gomega.Succeed())
	installCRDs(t, kubeClient, exampleRoot(t))

	return cluster, kubeClient, scheme
}

func assertModuleCRDsAbsent(t *testing.T, kubeClient client.Client, registry *modules.Registry) {
	t.Helper()
	g := gomega.NewWithT(t)

	for _, name := range registry.Names() {
		definition, _ := registry.Get(name)
		crd := new(apiextensionsv1.CustomResourceDefinition)
		err := kubeClient.Get(t.Context(), types.NamespacedName{Name: definition.CRDName}, crd)
		g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
	}
}

func waitForServingStatus(t *testing.T, kubeClient client.Client, kserveReady bool) {
	t.Helper()
	g := gomega.NewWithT(t)
	g.Eventually(func(g gomega.Gomega) {
		current := v1alpha1.NewServing()
		key := types.NamespacedName{Name: v1alpha1.InstanceName}
		g.Expect(kubeClient.Get(t.Context(), key, current)).To(gomega.Succeed())
		g.Expect(current.Status.Kserve).NotTo(gomega.BeNil())
		g.Expect(current.Status.MaaS).NotTo(gomega.BeNil())
		g.Expect(current.Status.Kserve.Ready).To(gomega.Equal(kserveReady))
		g.Expect(current.Status.MaaS.Ready).To(gomega.BeTrue())
	}).WithContext(t.Context()).WithTimeout(3 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
}

func waitForKserveRemoval(t *testing.T, kubeClient client.Client) {
	t.Helper()
	g := gomega.NewWithT(t)
	g.Eventually(func(g gomega.Gomega) {
		kserve := new(kservev1alpha1.Kserve)
		err := kubeClient.Get(t.Context(), types.NamespacedName{Name: v1alpha1.InstanceName}, kserve)
		g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())

		module := v1alpha1.NewPlatformModule()
		err = kubeClient.Get(t.Context(), types.NamespacedName{Name: "kserve"}, module)
		g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())

		deployment := new(appsv1.Deployment)
		key := types.NamespacedName{Name: "example-kserve", Namespace: "default"}
		err = kubeClient.Get(t.Context(), key, deployment)
		g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
	}).WithContext(t.Context()).WithTimeout(3 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
}

func createServing(t *testing.T, kubeClient client.Client) *v1alpha1.Serving {
	t.Helper()
	g := gomega.NewWithT(t)

	serving := v1alpha1.NewServing()
	serving.Name = v1alpha1.InstanceName
	serving.Spec.Kserve.ManagementState = "Managed"
	serving.Spec.MaaS.ManagementState = "Managed"
	g.Expect(kubeClient.Create(t.Context(), serving)).To(gomega.Succeed())

	return serving
}

func waitForModuleControllers(
	t *testing.T,
	kubeClient client.Client,
	registry *modules.Registry,
	image string,
	names ...string,
) {
	t.Helper()
	g := gomega.NewWithT(t)

	if len(names) == 0 {
		names = registry.Names()
	}
	for _, name := range names {
		definition, _ := registry.Get(name)
		g.Eventually(func(g gomega.Gomega) {
			crd := new(apiextensionsv1.CustomResourceDefinition)
			g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: definition.CRDName}, crd)).To(gomega.Succeed())

			module := v1alpha1.NewPlatformModule()
			g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: name}, module)).To(gomega.Succeed())
			g.Expect(module.Spec.Module).To(gomega.Equal(name))

			deployment := new(appsv1.Deployment)
			key := types.NamespacedName{Name: "example-" + name, Namespace: "default"}
			g.Expect(kubeClient.Get(t.Context(), key, deployment)).To(gomega.Succeed())
			g.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal(image))
			g.Expect(deployment.Status.AvailableReplicas).To(gomega.Equal(int32(1)))
		}).WithContext(t.Context()).WithTimeout(5 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
	}
}

func exampleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve example root")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func installCRDs(t *testing.T, kubeClient client.Client, root string) {
	t.Helper()
	g := gomega.NewWithT(t)
	for _, name := range []string{"platforms", "platformmodules", "servings"} {
		path := filepath.Join(root, "config", "crd", "bases", "example.platform.odh.io_"+name+".yaml")
		data, err := os.ReadFile(path) //nolint:gosec // The name comes from a fixed list of example CRDs.
		g.Expect(err).To(gomega.Succeed())

		crd := new(apiextensionsv1.CustomResourceDefinition)
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
		g.Expect(decoder.Decode(crd)).To(gomega.Succeed())
		g.Expect(kubeClient.Create(t.Context(), crd)).To(gomega.Succeed())

		g.Eventually(func(g gomega.Gomega) {
			current := new(apiextensionsv1.CustomResourceDefinition)
			g.Expect(kubeClient.Get(t.Context(), types.NamespacedName{Name: crd.Name}, current)).To(gomega.Succeed())
			g.Expect(current.Status.Conditions).To(gomega.ContainElement(gomega.Satisfy(
				func(condition apiextensionsv1.CustomResourceDefinitionCondition) bool {
					return condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue
				},
			)))
		}).WithContext(t.Context()).WithTimeout(2 * time.Minute).WithPolling(500 * time.Millisecond).Should(gomega.Succeed())
	}
}

func newManager(t *testing.T, config *rest.Config, scheme *k8sruntime.Scheme) manager.Manager {
	t.Helper()
	g := gomega.NewWithT(t)
	instance, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Controller: controllerconfig.Controller{
			SkipNameValidation: new(true),
		},
		Client: client.Options{Cache: &client.CacheOptions{
			Unstructured: true,
		}},
	})
	g.Expect(err).To(gomega.Succeed())

	return instance
}

func startManager(t *testing.T, instance manager.Manager) {
	t.Helper()
	g := gomega.NewWithT(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- instance.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		g.Expect(<-done).To(gomega.Succeed())
	})

	startup, cancelStartup := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancelStartup()
	g.Expect(instance.GetCache().WaitForCacheSync(startup)).To(gomega.BeTrue())
}

func runtimeUnavailable(err error) bool {
	message := strings.ToLower(err.Error())

	return strings.Contains(message, "failed to get docker info") ||
		strings.Contains(message, "cannot connect to the docker daemon") ||
		strings.Contains(message, "failed to get podman info")
}
