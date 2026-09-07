package validation_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/validation"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
)

func TestValidateRequiredAndOptionalPlatformCapabilities(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	g.Expect(validation.Validate(&consumer.Consumer{})).Should(Succeed())

	optional := &consumer.OptionalConsumer{}
	optional.SetPlatformProfile(api.PlatformProfile{Kind: "platform"})
	g.Expect(validation.Validate(optional)).Should(Succeed())
}
