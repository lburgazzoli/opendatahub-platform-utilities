package config_test

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"sync"
	"testing"

	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/config"
)

type settings struct {
	Tags  map[string]string
	Value string
}

func cloneSettings(value settings) settings {
	cloned := value

	cloned.Tags = make(map[string]string, len(value.Tags))
	maps.Copy(cloned.Tags, value.Tags)

	return cloned
}

func TestLoaderAppliesFileThenEnvironmentAndClonesValues(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	filesystem := fstest.MapFS{"config": &fstest.MapFile{Data: []byte("file")}}
	fileSource, err := config.NewFileSource(filesystem, "config", func(data []byte, target *settings) error {
		target.Value = string(data)
		target.Tags["source"] = "file"

		return nil
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	environmentSource, err := config.NewEnvironmentSourceWithLookup(
		[]string{"VALUE"},
		func(key string) (string, bool) { return "environment", key == "VALUE" },
		func(values map[string]string, target *settings) error {
			target.Value = values["VALUE"]
			target.Tags["source"] = "environment"

			return nil
		},
	)
	g.Expect(err).ShouldNot(HaveOccurred())
	loader, err := config.New(
		settings{Value: "default", Tags: map[string]string{"default": "yes"}},
		cloneSettings,
		fileSource,
		environmentSource,
	)
	g.Expect(err).ShouldNot(HaveOccurred())

	got, err := loader.Load(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(got.Value).Should(Equal("environment"))
	g.Expect(got.Tags).Should(HaveKeyWithValue("source", "environment"))

	got.Tags["source"] = "caller"
	second, err := loader.Load(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(second.Tags).Should(HaveKeyWithValue("source", "environment"))
}

func TestLoaderIsSafeForConcurrentLoads(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	source := config.SourceFunc[settings](func(_ context.Context, target *settings) error {
		target.Tags["source"] = "loader"

		return nil
	})
	loader, err := config.New(settings{Tags: map[string]string{}}, cloneSettings, source)
	g.Expect(err).ShouldNot(HaveOccurred())

	var group sync.WaitGroup

	results := make(chan settings, 8)

	for range 8 {
		group.Go(func() {
			value, loadErr := loader.Load(t.Context())
			if loadErr == nil {
				results <- value
			}
		})
	}

	group.Wait()
	close(results)

	for result := range results {
		g.Expect(result.Tags).Should(HaveKeyWithValue("source", "loader"))
	}
}

func TestSourceErrorsAreWrapped(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	filesystem := fstest.MapFS{}
	fileSource, err := config.NewFileSource(filesystem, "missing", func([]byte, *settings) error {
		return nil
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	loader, err := config.New(settings{}, cloneSettings, fileSource)
	g.Expect(err).ShouldNot(HaveOccurred())

	_, err = loader.Load(t.Context())
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err).Should(MatchError(ContainSubstring("configuration source 0")))
	g.Expect(errors.Is(err, fs.ErrNotExist)).Should(BeTrue())
}
