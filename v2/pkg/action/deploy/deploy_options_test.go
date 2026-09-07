package deploy_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

func TestRunRejectsInvalidStableOptions(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := deploy.New(option.FunctionalOption[deploy.Options](func(options *deploy.Options) {
		options.FieldOwner = nil
	}))

	g.Expect(action.Validate()).Should(MatchError(ContainSubstring("field owner is required")))

	_, err := action.Run(t.Context())
	g.Expect(err).Should(MatchError(ContainSubstring("field owner is required")))
}

func TestRunRejectsNilSortFunction(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := deploy.New(option.FunctionalOption[deploy.Options](func(options *deploy.Options) {
		options.Sort = nil
	}))

	g.Expect(action.Validate()).Should(MatchError("deploy sort function is required"))
}
