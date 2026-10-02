package kind

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrAlreadyStarted    = errors.New("kind engine already started")
	ErrClusterNotStarted = errors.New("kind cluster is not started")
	ErrLogsDirRequired   = errors.New("kind logs directory is required")
)

// Engine owns one disposable Kind cluster at a time.
//
//nolint:govet // Keep the provider and its cluster state together for lifecycle review.
type Engine struct {
	provider        provider
	providerFactory providerFactory
	cluster         *Cluster
	tempDir         string
	pendingTempDir  string
	options         Options
	mu              sync.Mutex
	removeTempDir   func(string) error
}

// New constructs an engine. Validation and provider detection happen in Start
// so construction remains safe for test setup and option inspection.
func New(options ...Option) *Engine {
	return &Engine{
		options:         applyOptions(defaultOptions(), options...),
		providerFactory: newProvider,
		removeTempDir:   removeTempDir,
	}
}

// Cluster represents a started Kind cluster and exposes independent client
// configuration to callers.
type Cluster struct {
	config         *rest.Config
	name           string
	kubeconfigPath string
}

// Name returns the Kind cluster name.
func (c *Cluster) Name() string { return c.name }

// KubeconfigPath returns the kubeconfig path used by the cluster.
func (c *Cluster) KubeconfigPath() string { return c.kubeconfigPath }

// RESTConfig returns a defensive copy of the cluster REST configuration.
func (c *Cluster) RESTConfig() *rest.Config { return rest.CopyConfig(c.config) }

// Client creates a controller-runtime client using the supplied scheme.
func (c *Cluster) Client(scheme *runtime.Scheme) (client.Client, error) {
	if scheme == nil {
		return nil, ErrSchemeRequired
	}

	return client.New(c.RESTConfig(), client.Options{Scheme: scheme})
}

// Start creates a cluster through the Kind Go provider, exports its kubeconfig,
// and waits until the API server answers successfully. If startup fails after
// creation, the engine attempts to delete the partial cluster before returning
// the error.
//
//nolint:cyclop // Startup checks each Kind lifecycle stage.
func (e *Engine) Start(ctx context.Context) (*Cluster, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cluster != nil || e.pendingTempDir != "" {
		return nil, ErrAlreadyStarted
	}

	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	options, err := e.resolvedOptions()
	if err != nil {
		return nil, err
	}

	kindProvider, err := e.providerFactory(options)
	if err != nil {
		return nil, fmt.Errorf("create Kind provider: %w", err)
	}

	kubeconfigPath, tempDir, err := e.prepareKubeconfig(options)
	if err != nil {
		return nil, fmt.Errorf("prepare kubeconfig: %w", err)
	}

	createOptions := createOptions(options)

	err = kindProvider.Create(options.Name, createOptions...)
	if err != nil {
		tempErr := e.removeTempDir(tempDir)

		return nil, errors.Join(fmt.Errorf("create cluster %q: %w", options.Name, err), tempErr)
	}

	err = kindProvider.ExportKubeConfig(options.Name, kubeconfigPath, false)
	if err != nil {
		return nil, e.startFailure(ctx, kindProvider, options, tempDir, fmt.Errorf("export kubeconfig: %w", err))
	}

	config, err := loadRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, e.startFailure(ctx, kindProvider, options, tempDir, fmt.Errorf("load kubeconfig: %w", err))
	}

	err = waitForReady(ctx, config)
	if err != nil {
		return nil, e.startFailure(ctx, kindProvider, options, tempDir, fmt.Errorf("wait for API server: %w", err))
	}

	e.provider = kindProvider
	e.tempDir = tempDir
	e.cluster = &Cluster{
		config:         config,
		name:           options.Name,
		kubeconfigPath: kubeconfigPath,
	}

	return e.cluster, nil
}

// Close deletes the cluster unless Keep was configured. It is safe to call
// Close more than once.
func (e *Engine) Close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cluster == nil {
		return e.cleanupPendingTempDir(ctx)
	}

	err := ctx.Err()
	if err != nil {
		return err
	}

	cluster := e.cluster
	kindProvider := e.provider
	tempDir := e.tempDir

	if e.keepCluster() {
		return nil
	}

	deleteErr := kindProvider.Delete(cluster.name, cluster.kubeconfigPath)

	tempErr := e.removeTempDirectory(tempDir)
	if tempErr == nil {
		e.tempDir = ""
	}

	if deleteErr == nil {
		e.cluster = nil
		e.provider = nil
		e.tempDir = ""
	}

	return errors.Join(deleteErr, tempErr)
}

// CollectLogs saves node logs for a started cluster, typically before Close
// when an integration test fails.
func (e *Engine) CollectLogs(dir string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cluster == nil {
		return ErrClusterNotStarted
	}

	if dir == "" {
		return ErrLogsDirRequired
	}

	err := e.provider.CollectLogs(e.cluster.name, dir)
	if err != nil {
		return fmt.Errorf("collect Kind logs: %w", err)
	}

	return nil
}

func (e *Engine) cleanupPendingTempDir(ctx context.Context) error {
	if e.pendingTempDir == "" {
		return nil
	}

	err := ctx.Err()
	if err != nil {
		return err
	}

	err = e.removeTempDirectory(e.pendingTempDir)
	if err != nil {
		return err
	}

	e.pendingTempDir = ""

	return nil
}

func (e *Engine) removeTempDirectory(path string) error {
	if path == "" {
		e.pendingTempDir = ""

		return nil
	}

	err := e.removeTempDir(path)
	if err != nil {
		e.pendingTempDir = path

		return err
	}

	e.pendingTempDir = ""

	return nil
}
