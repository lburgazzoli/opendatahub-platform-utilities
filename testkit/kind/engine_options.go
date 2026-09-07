package kind

import (
	"slices"
	"time"

	kindcluster "sigs.k8s.io/kind/pkg/cluster"
)

const defaultWait = 2 * time.Minute

// Options configures an Engine.
type Options struct {
	Name            string
	NodeImage       string
	KubeconfigPath  string
	Wait            *time.Duration
	Keep            *bool
	ProviderOptions []kindcluster.ProviderOption
}

// Option changes engine configuration before startup.
type Option interface {
	ApplyTo(options *Options)
}

// FunctionalOption adapts a function to Option.
type FunctionalOption func(*Options)

// ApplyTo applies a functional option.
func (f FunctionalOption) ApplyTo(options *Options) {
	f(options)
}

// ApplyTo applies the present fields of a complete options value and clones
// caller-owned pointers and slices.
func (o Options) ApplyTo(target *Options) {
	if o.Name != "" {
		target.Name = o.Name
	}

	if o.NodeImage != "" {
		target.NodeImage = o.NodeImage
	}

	if o.KubeconfigPath != "" {
		target.KubeconfigPath = o.KubeconfigPath
	}

	if o.Wait != nil {
		target.Wait = new(*o.Wait)
	}

	if o.Keep != nil {
		target.Keep = new(*o.Keep)
	}

	if o.ProviderOptions != nil {
		target.ProviderOptions = slices.Clone(o.ProviderOptions)
	}
}

// WithName sets the Kind cluster name.
func WithName(name string) Option {
	return FunctionalOption(func(options *Options) { options.Name = name })
}

// WithNodeImage sets the node image passed to the Kind provider.
func WithNodeImage(image string) Option {
	return FunctionalOption(func(options *Options) { options.NodeImage = image })
}

// WithKubeconfigPath writes the cluster kubeconfig to path instead of a
// temporary directory owned by the engine.
func WithKubeconfigPath(path string) Option {
	return FunctionalOption(func(options *Options) { options.KubeconfigPath = path })
}

// WithWait sets the maximum duration used by the Kind provider while creating
// the cluster.
func WithWait(wait time.Duration) Option {
	return FunctionalOption(func(options *Options) { options.Wait = new(wait) })
}

// WithKeep leaves the cluster and an engine-owned kubeconfig in place when
// Close is called. This is useful for debugging failed integration tests.
func WithKeep(keep bool) Option {
	return FunctionalOption(func(options *Options) { options.Keep = new(keep) })
}

// WithProviderOption adds a provider option from the Kind library. Supplying a
// provider option disables automatic node-provider detection.
func WithProviderOption(providerOption kindcluster.ProviderOption) Option {
	return FunctionalOption(func(options *Options) {
		if providerOption != nil {
			options.ProviderOptions = append(options.ProviderOptions, providerOption)
		}
	})
}

// WithDocker selects Docker as the Kind node provider.
func WithDocker() Option {
	return WithProviderOption(kindcluster.ProviderWithDocker())
}

// WithPodman selects Podman as the Kind node provider.
func WithPodman() Option {
	return WithProviderOption(kindcluster.ProviderWithPodman())
}

func defaultOptions() Options {
	return Options{Wait: new(defaultWait), Keep: new(false)}
}

func applyOptions(options Options, provided ...Option) Options {
	for _, option := range provided {
		if option != nil {
			option.ApplyTo(&options)
		}
	}

	return cloneOptions(options)
}

func cloneOptions(options Options) Options {
	clone := options
	if options.Wait != nil {
		clone.Wait = new(*options.Wait)
	}

	if options.Keep != nil {
		clone.Keep = new(*options.Keep)
	}

	clone.ProviderOptions = slices.Clone(options.ProviderOptions)

	return clone
}

var (
	_ Option = Options{}
	_ Option = FunctionalOption(nil)
)
