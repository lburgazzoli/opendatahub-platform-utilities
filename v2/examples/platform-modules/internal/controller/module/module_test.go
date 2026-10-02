package module

import (
	"testing"

	"github.com/onsi/gomega"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	aigatewayv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/aigateway/v1alpha1"
	kservev1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/kserve/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

func TestUpdateStatusFromFailureAnnotation(t *testing.T) {
	t.Parallel()

	modules := map[string]platformapi.PlatformObject{
		"kserve":    kservev1alpha1.NewKserve(),
		"aigateway": aigatewayv1alpha1.NewAIGateway(),
	}
	for name, object := range modules {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			controller := &ModuleController{}
			request := &pipeline.Request{Instance: object}
			object.SetAnnotations(map[string]string{SimulationAnnotation: "dependency unavailable"})

			g.Expect(controller.updateStatus(t.Context(), request)).To(gomega.MatchError(
				gomega.ContainSubstring("dependency unavailable"),
			))
			g.Expect(condition.IsTrue(object.GetStatus(), ConditionModuleConfigured)).To(gomega.BeTrue())
			g.Expect(condition.IsTrue(object.GetStatus(), ConditionSimulationActive)).To(gomega.BeTrue())

			object.SetAnnotations(map[string]string{SimulationAnnotation: ""})
			g.Expect(controller.updateStatus(t.Context(), request)).To(gomega.Succeed())
			g.Expect(condition.IsFalse(object.GetStatus(), ConditionSimulationActive)).To(gomega.BeTrue())

			object.SetAnnotations(nil)
			g.Expect(controller.updateStatus(t.Context(), request)).To(gomega.Succeed())
			g.Expect(condition.IsFalse(object.GetStatus(), ConditionSimulationActive)).To(gomega.BeTrue())
		})
	}
}
