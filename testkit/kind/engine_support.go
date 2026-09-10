package kind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kindcluster "sigs.k8s.io/kind/pkg/cluster"
)

const defaultNamePrefix = "odh-v2"

var (
	ErrSchemeRequired      = errors.New("scheme is required")
	ErrClusterNameRequired = errors.New("cluster name is required")
	ErrInvalidWait         = errors.New("wait duration must be positive")
)

type effectiveOptions struct {
	Name            string
	NodeImage       string
	KubeconfigPath  string
	ProviderOptions []kindcluster.ProviderOption
	Wait            time.Duration
	Keep            bool
}

func (e *Engine) resolvedOptions() (effectiveOptions, error) {
	options := effectiveOptions{
		Name:            e.options.Name,
		NodeImage:       e.options.NodeImage,
		KubeconfigPath:  e.options.KubeconfigPath,
		Wait:            defaultWait,
		ProviderOptions: slices.Clone(e.options.ProviderOptions),
	}
	if e.options.Wait != nil {
		options.Wait = *e.options.Wait
	}

	if e.options.Keep != nil {
		options.Keep = *e.options.Keep
	}

	if options.Name == "" {
		options.Name = fmt.Sprintf("%s-%d", defaultNamePrefix, os.Getpid())
	}

	if strings.TrimSpace(options.Name) == "" {
		return effectiveOptions{}, ErrClusterNameRequired
	}

	if options.Wait <= 0 {
		return effectiveOptions{}, ErrInvalidWait
	}

	return options, nil
}

func createOptions(options effectiveOptions) []kindcluster.CreateOption {
	createOptions := []kindcluster.CreateOption{
		kindcluster.CreateWithWaitForReady(options.Wait),
	}
	if options.NodeImage != "" {
		createOptions = append(createOptions, kindcluster.CreateWithNodeImage(options.NodeImage))
	}

	return createOptions
}

func (e *Engine) prepareKubeconfig(options effectiveOptions) (string, string, error) {
	if options.KubeconfigPath != "" {
		path := filepath.Clean(options.KubeconfigPath)

		err := os.MkdirAll(filepath.Dir(path), 0o750)
		if err != nil {
			return "", "", err
		}

		return path, "", nil
	}

	tempDir, err := os.MkdirTemp("", "odh-kind-")
	if err != nil {
		return "", "", err
	}

	return filepath.Join(tempDir, "kubeconfig"), tempDir, nil
}

func (e *Engine) startFailure(
	ctx context.Context,
	provider provider,
	options effectiveOptions,
	tempDir string,
	startErr error,
) error {
	deleteErr := provider.Delete(options.Name, "")
	if ctx.Err() != nil {
		deleteErr = errors.Join(deleteErr, ctx.Err())
	}

	tempErr := e.removeTempDirectory(tempDir)

	return errors.Join(startErr, deleteErr, tempErr)
}

func newProvider(options effectiveOptions) (provider, error) {
	providerOptions := options.ProviderOptions
	if len(providerOptions) == 0 {
		detected, err := kindcluster.DetectNodeProvider()
		if err != nil {
			return nil, err
		}

		providerOptions = []kindcluster.ProviderOption{detected}
	}

	return kindcluster.NewProvider(providerOptions...), nil
}

func (e *Engine) keepCluster() bool {
	return e.options.Keep != nil && *e.options.Keep
}

func loadRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

func waitForReady(ctx context.Context, config *rest.Config) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	interval := time.NewTicker(500 * time.Millisecond)
	defer interval.Stop()

	for {
		_, err := clientset.Discovery().ServerVersion()
		if err == nil {
			return nil
		}

		if !apierrors.IsServiceUnavailable(err) && !apierrors.IsTimeout(err) && !apierrors.IsInternalError(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-interval.C:
		}
	}
}

func removeTempDir(path string) error {
	if path == "" {
		return nil
	}

	return os.RemoveAll(path)
}

type provider interface {
	Create(name string, options ...kindcluster.CreateOption) error
	Delete(name, kubeconfigPath string) error
	ExportKubeConfig(name, path string, internal bool) error
}

type providerFactory func(effectiveOptions) (provider, error)
