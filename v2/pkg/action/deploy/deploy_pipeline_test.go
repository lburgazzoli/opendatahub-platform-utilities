package deploy

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

func TestRunOptionsFromRequestUsesFieldOwnerExtension(t *testing.T) {
	t.Parallel()

	request := &pipeline.Request{
		Extensions: pipeline.Extension{
			pipeline.ExtensionFieldOwner: "controller-owner",
		},
	}

	g := NewWithT(t)
	options := runOptionsFromRequest(request)

	g.Expect(options.FieldOwner).Should(Equal("controller-owner"))
}
