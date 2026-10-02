package deploy

import (
	"fmt"
	"maps"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunOptions contains values that vary for each deployment invocation.
type RunOptions struct {
	Client     client.Client
	Owner      client.Object
	Resources  resources.Accessor
	FieldOwner string
	// Labels and Annotations replace earlier per-run maps when non-nil. They
	// override constructor metadata, before the metadata policy is applied.
	Labels      map[string]string
	Annotations map[string]string
}

// ApplyTo copies invocation values into target. Nil maps and interfaces leave
// target unchanged; non-nil maps replace earlier per-run values, including
// when empty. This allows composition with functional options.
func (o RunOptions) ApplyTo(target *RunOptions) {
	if o.Client != nil {
		target.Client = o.Client
	}
	if o.Owner != nil {
		target.Owner = o.Owner
	}
	if o.Resources != nil {
		target.Resources = o.Resources
	}
	if o.FieldOwner != "" {
		target.FieldOwner = o.FieldOwner
	}
	if o.Labels != nil {
		target.Labels = maps.Clone(o.Labels)
	}
	if o.Annotations != nil {
		target.Annotations = maps.Clone(o.Annotations)
	}
}

// RunOption configures one invocation.
type RunOption = option.Option[RunOptions]

// WithRunClient sets the Kubernetes client for this invocation.
func WithRunClient(kubernetesClient client.Client) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if kubernetesClient != nil {
			options.Client = kubernetesClient
		}
	})
}

// WithRunOwner sets the owner for this invocation.
func WithRunOwner(owner client.Object) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if owner != nil {
			options.Owner = owner
		}
	})
}

// WithRunResources sets the desired resource collection for this invocation.
func WithRunResources(accessor resources.Accessor) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if accessor != nil {
			options.Resources = accessor
		}
	})
}

// WithRunFieldOwner sets the SSA field owner for this invocation. An empty
// value leaves the configured fallback unchanged.
func WithRunFieldOwner(owner string) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if owner != "" {
			options.FieldOwner = owner
		}
	})
}

// WithRunLabel adds or replaces one label for this invocation.
func WithRunLabel(key string, value string) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if options.Labels == nil {
			options.Labels = make(map[string]string)
		}
		options.Labels[key] = value
	})
}

// WithRunLabels merges labels for this invocation.
func WithRunLabels(values map[string]string) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if values == nil {
			return
		}
		if options.Labels == nil {
			options.Labels = make(map[string]string)
		}
		maps.Copy(options.Labels, values)
	})
}

// WithRunAnnotation adds or replaces one annotation for this invocation.
func WithRunAnnotation(key string, value string) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if options.Annotations == nil {
			options.Annotations = make(map[string]string)
		}
		options.Annotations[key] = value
	})
}

// WithRunAnnotations merges annotations for this invocation.
func WithRunAnnotations(values map[string]string) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		if values == nil {
			return
		}
		if options.Annotations == nil {
			options.Annotations = make(map[string]string)
		}
		maps.Copy(options.Annotations, values)
	})
}

// Merge applies invocation options from left to right without changing
// caller-owned maps already held by the receiver.
func (o *RunOptions) Merge(values ...RunOption) {
	o.Labels = maps.Clone(o.Labels)
	o.Annotations = maps.Clone(o.Annotations)

	for _, value := range values {
		if value != nil {
			value.ApplyTo(o)
		}
	}
}

// Validate checks the required inputs before deployment begins.
func (o RunOptions) Validate() error {
	if o.Client == nil {
		return fmt.Errorf("deploy client: %w", ErrRunInputRequired)
	}
	if o.Owner == nil {
		return fmt.Errorf("deploy owner: %w", ErrRunInputRequired)
	}
	if o.Resources == nil {
		return fmt.Errorf("deploy resources: %w", ErrRunInputRequired)
	}

	return nil
}

var _ RunOption = RunOptions{}
