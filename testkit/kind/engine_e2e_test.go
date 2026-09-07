//go:build e2e

package kind

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	kindcluster "sigs.k8s.io/kind/pkg/cluster"
)

func TestEngineCreatesAndDeletesCluster(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	if _, err := kindcluster.DetectNodeProvider(); err != nil {
		t.Skipf("Kind is unavailable: %v", err)
	}

	engine := New(WithName("odh-v2-engine-test"))
	cluster, err := engine.Start(t.Context())
	if err != nil {
		if runtimeUnavailable(err) {
			t.Skipf("container runtime is unavailable: %v", err)
		}
		g.Expect(err).ShouldNot(HaveOccurred())
	}
	t.Cleanup(func() {
		NewWithT(t).Expect(engine.Close(t.Context())).Should(Succeed())
	})

	g.Expect(cluster.Name()).ShouldNot(BeEmpty())
	g.Expect(cluster.KubeconfigPath()).ShouldNot(BeEmpty())
	_, err = cluster.Client(runtime.NewScheme())
	g.Expect(err).ShouldNot(HaveOccurred())
}

func runtimeUnavailable(err error) bool {
	message := strings.ToLower(err.Error())

	return strings.Contains(message, "failed to get docker info") ||
		strings.Contains(message, "cannot connect to the docker daemon") ||
		strings.Contains(message, "failed to get podman info")
}
