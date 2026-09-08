package integration_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/testkit/integration"
)

func TestNewFileSourceAcceptsAbsolutePath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	source, err := integration.NewFileSource("/tmp/manifest.yaml")
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(source.Path).Should(Equal("tmp/manifest.yaml"))
}

func TestNewFileSourceRequiresAbsolutePath(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	_, err := integration.NewFileSource("manifest.yaml")
	g.Expect(err).Should(MatchError(ContainSubstring("path must be absolute")))
}

func TestNewURLSourceRequiresHTTPS(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	_, err := integration.NewURLSource("http://example.test/manifest.yaml")
	g.Expect(err).Should(MatchError(ContainSubstring("URL scheme must be HTTPS")))
}

func TestURLSourceLoadsManifest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	source, err := integration.NewURLSource("https://example.test/manifest.yaml")
	g.Expect(err).ShouldNot(HaveOccurred())

	client := new(http.Client)
	client.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		response := new(http.Response)
		response.StatusCode = http.StatusOK
		response.Body = io.NopCloser(strings.NewReader("kind: ConfigMap\n"))

		return response, nil
	})
	source.Client = client

	data, err := source.Load(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(string(data)).Should(Equal("kind: ConfigMap\n"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
