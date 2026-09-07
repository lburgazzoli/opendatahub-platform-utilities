package release

import (
	"io/fs"
	"os"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

// Options configures release metadata discovery.
type Options struct {
	FS   fs.FS
	Path string
}

// ApplyTo copies release metadata discovery.
func (o Options) ApplyTo(target *Options) {
	target.FS = o.FS
	target.Path = o.Path
}

// Option configures release metadata discovery.
type Option = option.Option[Options]

// WithFS supplies the metadata filesystem.
func WithFS(fsys fs.FS) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.FS = fsys
	})
}

// WithPath supplies the metadata file path.
func WithPath(path string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Path = path
	})
}

func defaultOptions() Options {
	return Options{FS: os.DirFS("/"), Path: ComponentMetadataFilename}
}

var _ Option = Options{}
