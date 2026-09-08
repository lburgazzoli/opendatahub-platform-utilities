package reconciler

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

func TestInstance(t *testing.T) {
	t.Parallel()

	instance := testObjectInstance("component")
	request := &pipeline.Request{Instance: instance}

	g := NewWithT(t)
	actual, err := Instance[*testObject](request)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(actual).Should(BeIdenticalTo(instance))
}

func TestInstanceRejectsUnexpectedType(t *testing.T) {
	t.Parallel()

	request := &pipeline.Request{Instance: testObjectInstance("component")}

	g := NewWithT(t)
	actual, err := Instance[*instanceTestObject](request)

	g.Expect(actual).Should(BeNil())
	g.Expect(err).Should(MatchError(MatchRegexp("reconciler request instance has unexpected type")))
}

func TestInstanceRejectsNilRequest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	actual, err := Instance[*testObject](nil)

	g.Expect(actual).Should(BeNil())
	g.Expect(err).Should(MatchError(MatchRegexp("reconciler request instance has unexpected type: request is nil")))
}

type instanceTestObject struct {
	testObject
}
