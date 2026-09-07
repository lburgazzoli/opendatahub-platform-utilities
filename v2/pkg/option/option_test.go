package option_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

type options struct {
	Value string
}

func (o options) ApplyTo(target *options) {
	target.Value = o.Value
}

func TestCompleteAndFunctionalOptionsShareContract(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var (
		complete   option.Option[options] = options{Value: "complete"}
		functional option.Option[options] = option.FunctionalOption[options](func(target *options) {
			target.Value = "functional"
		})
	)

	got := options{}
	complete.ApplyTo(&got)
	g.Expect(got.Value).Should(Equal("complete"))
	functional.ApplyTo(&got)
	g.Expect(got.Value).Should(Equal("functional"))
}
