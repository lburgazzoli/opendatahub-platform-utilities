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
	ErrAlreadyStarted = errors.New("kind engine already started")
)

// Engine owns one disposable Kind cluster at a time.
type Engine struct {
	provider        provider
	providerFactory providerFactory
	cluster         *Cluster
	tempDir         string
	options         Options
	mu              sync.Mutex
}

// New constructs an engine. Validation and provider detection happen in Start
// so construction remains safe for test setup and option inspection.
func New(options ...Option) *Engine {
	return &Engine{
		options:         applyOptions(defaultOptions(), options...),
		providerFactory: newProvider,
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
func (e *Engine) Start(ctx context.Context) (*Cluster, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cluster != nil {
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
		_ = removeTempDir(tempDir)

		return nil, fmt.Errorf("create cluster %q: %w", options.Name, err)
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
		return nil
	}

	err := ctx.Err()
	if err != nil {
		return err
	}

	cluster := e.cluster
	kindProvider := e.provider
	tempDir := e.tempDir
	e.cluster = nil
	e.provider = nil
	e.tempDir = ""

	if e.keepCluster() {
		return nil
	}

	deleteErr := kindProvider.Delete(cluster.name, cluster.kubeconfigPath)

	return errors.Join(deleteErr, removeTempDir(tempDir))
}
