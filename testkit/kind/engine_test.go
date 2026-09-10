package kind

import (
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	kindcluster "sigs.k8s.io/kind/pkg/cluster"
)

var (
	errProviderDelete = errors.New("provider delete failed")             //nolint:gochecknoglobals,err113 // Shared test outcome.
	errTempCleanup    = errors.New("temporary directory cleanup failed") //nolint:gochecknoglobals,err113 // Shared test outcome.
)

func TestNewAppliesDefaultsAndOptions(t *testing.T) {
	t.Parallel()

	engine := New(
		WithName("test-cluster"),
		WithNodeImage("kindest/node:v1.35.0"),
		WithWait(3*time.Minute),
		WithKeep(true),
		WithDocker(),
	)

	g := NewWithT(t)
	g.Expect(engine.options.Name).Should(Equal("test-cluster"))
	g.Expect(engine.options.NodeImage).Should(Equal("kindest/node:v1.35.0"))
	g.Expect(engine.options.Wait).ShouldNot(BeNil())
	g.Expect(*engine.options.Wait).Should(Equal(3 * time.Minute))
	g.Expect(engine.options.Keep).ShouldNot(BeNil())
	g.Expect(*engine.options.Keep).Should(BeTrue())
	g.Expect(engine.options.ProviderOptions).Should(HaveLen(1))
}

func TestResolvedOptionsUsesProcessScopedDefaultName(t *testing.T) {
	t.Parallel()

	engine := New()
	options, err := engine.resolvedOptions()
	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(options.Name).ShouldNot(BeEmpty())
}

func TestResolvedOptionsRejectsInvalidWait(t *testing.T) {
	t.Parallel()

	_, err := New(WithWait(0)).resolvedOptions()
	g := NewWithT(t)
	g.Expect(err).Should(MatchError(ContainSubstring("wait duration")))
}

func TestStartFailureDeletesPartialCluster(t *testing.T) {
	t.Parallel()

	recorder := new(providerMock)
	recorder.On("Delete", "partial", "").Return(nil).Once()

	engine := New()
	err := engine.startFailure(
		t.Context(),
		recorder,
		effectiveOptions{Name: "partial"},
		"",
		fmt.Errorf("startup failed: %w", ErrInvalidWait),
	)
	g := NewWithT(t)
	g.Expect(err).Should(MatchError(ContainSubstring("startup failed")))
	g.Expect(recorder.AssertExpectations(t)).Should(BeTrue())
}

func TestCompleteOptionsImplementsOption(t *testing.T) {
	t.Parallel()

	wait := 3 * time.Minute
	keep := true
	engine := New(Options{
		Name: "struct-options",
		Wait: &wait,
		Keep: &keep,
	})

	g := NewWithT(t)
	g.Expect(engine.options.Name).Should(Equal("struct-options"))
	g.Expect(engine.options.Wait).ShouldNot(BeNil())
	g.Expect(*engine.options.Wait).Should(Equal(wait))
	g.Expect(engine.options.Keep).ShouldNot(BeNil())
	g.Expect(*engine.options.Keep).Should(BeTrue())
}

func TestCloseDeletesStartedCluster(t *testing.T) {
	t.Parallel()

	recorder := new(providerMock)
	recorder.On("Delete", "cluster", "/tmp/kubeconfig").Return(nil).Once()

	engine := New()
	engine.provider = recorder
	engine.cluster = &Cluster{name: "cluster", kubeconfigPath: "/tmp/kubeconfig"}

	g := NewWithT(t)
	g.Expect(engine.Close(t.Context())).Should(Succeed())
	g.Expect(recorder.AssertExpectations(t)).Should(BeTrue())
	g.Expect(engine.Close(t.Context())).Should(Succeed())
}

func TestCloseKeepPreservesCluster(t *testing.T) {
	t.Parallel()

	recorder := new(providerMock)
	engine := New(WithKeep(true))
	engine.provider = recorder
	engine.cluster = &Cluster{name: "cluster", kubeconfigPath: "/tmp/kubeconfig"}

	g := NewWithT(t)
	g.Expect(engine.Close(t.Context())).Should(Succeed())
	g.Expect(recorder.AssertExpectations(t)).Should(BeTrue())
}

func TestCloseRetainsClusterWhenProviderDeletionFails(t *testing.T) {
	t.Parallel()

	recorder := new(providerMock)
	recorder.On("Delete", "cluster", "/tmp/kubeconfig").Return(errProviderDelete).Once()
	recorder.On("Delete", "cluster", "/tmp/kubeconfig").Return(nil).Once()

	engine := New()
	engine.provider = recorder
	engine.cluster = &Cluster{name: "cluster", kubeconfigPath: "/tmp/kubeconfig"}
	engine.removeTempDir = func(string) error { return nil }

	g := NewWithT(t)
	err := engine.Close(t.Context())
	g.Expect(err).Should(MatchError(errProviderDelete))
	g.Expect(engine.cluster).ShouldNot(BeNil())

	g.Expect(engine.Close(t.Context())).Should(Succeed())
	g.Expect(engine.cluster).Should(BeNil())
	g.Expect(recorder.AssertExpectations(t)).Should(BeTrue())
}

func TestCloseRetriesTemporaryDirectoryCleanup(t *testing.T) {
	t.Parallel()

	recorder := new(providerMock)
	recorder.On("Delete", "cluster", "/tmp/kubeconfig").Return(nil).Once()
	cleanupCalls := 0

	engine := New()
	engine.provider = recorder
	engine.cluster = &Cluster{name: "cluster", kubeconfigPath: "/tmp/kubeconfig"}
	engine.tempDir = "/tmp/kind"
	engine.removeTempDir = func(string) error {
		cleanupCalls++
		if cleanupCalls == 1 {
			return errTempCleanup
		}

		return nil
	}

	g := NewWithT(t)
	err := engine.Close(t.Context())
	g.Expect(err).Should(MatchError(errTempCleanup))
	g.Expect(engine.cluster).Should(BeNil())
	g.Expect(engine.pendingTempDir).Should(Equal("/tmp/kind"))

	g.Expect(engine.Close(t.Context())).Should(Succeed())
	g.Expect(engine.pendingTempDir).Should(BeEmpty())
	g.Expect(cleanupCalls).Should(Equal(2))
	g.Expect(recorder.AssertExpectations(t)).Should(BeTrue())
}

func TestStartFailurePreservesProviderAndTemporaryDirectoryErrors(t *testing.T) {
	t.Parallel()

	recorder := new(providerMock)
	recorder.On("Delete", "partial", "").Return(errProviderDelete).Once()
	engine := New()
	engine.removeTempDir = func(string) error { return errTempCleanup }
	startErr := fmt.Errorf("startup failed: %w", ErrInvalidWait)

	err := engine.startFailure(
		t.Context(),
		recorder,
		effectiveOptions{Name: "partial"},
		"/tmp/kind",
		startErr,
	)

	g := NewWithT(t)
	g.Expect(err).Should(MatchError(ContainSubstring("startup failed")))
	g.Expect(errors.Is(err, errProviderDelete)).Should(BeTrue())
	g.Expect(errors.Is(err, errTempCleanup)).Should(BeTrue())
	g.Expect(engine.pendingTempDir).Should(Equal("/tmp/kind"))
	g.Expect(recorder.AssertExpectations(t)).Should(BeTrue())
}

type providerMock struct{ mock.Mock }

func (p *providerMock) Create(name string, options ...kindcluster.CreateOption) error {
	args := p.Called(name, options)
	return args.Error(0)
}

func (p *providerMock) Delete(name, kubeconfigPath string) error {
	args := p.Called(name, kubeconfigPath)
	return args.Error(0)
}

func (p *providerMock) ExportKubeConfig(name, path string, internal bool) error {
	args := p.Called(name, path, internal)
	return args.Error(0)
}
