package api_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	api "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWireJSONCompatibility(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	condition := api.Condition{
		LastTransitionTime: metav1.Time{},
		Type:               string(api.ConditionTypeReady),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: 3,
	}
	status := struct {
		api.PhaseStatus
		api.ReleaseStatus
		api.Status
	}{
		Status: api.Status{Conditions: []api.Condition{condition}, ObservedGeneration: 3},
		ReleaseStatus: api.ReleaseStatus{Releases: []api.ComponentRelease{{
			Name:    "platform",
			Version: "2.0.0",
		}}},
		PhaseStatus: api.PhaseStatus{Phase: api.PhaseReady},
	}

	encoded, err := json.Marshal(status)
	g.Expect(err).ShouldNot(HaveOccurred())

	expected := `{"conditions":[{"lastTransitionTime":null,"type":"Ready","status":"True","observedGeneration":3}],
		"observedGeneration":3,"releases":[{"name":"platform","version":"2.0.0"}],"phase":"Ready"}`
	g.Expect(encoded).Should(MatchJSON(expected))
}

func TestPlatformProfileDeepCopyClonesAnnotations(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	profile := &api.PlatformProfile{
		Kind:         "ODH",
		Version:      "2.0.0",
		Distribution: api.Distribution{Kind: "Kubernetes", Version: "1.35"},
		Annotations:  map[string]string{"example.io/fact": "value"},
	}

	cloned := profile.DeepCopy()
	cloned.Annotations["example.io/fact"] = "changed"

	g.Expect(profile.Annotations).Should(HaveKeyWithValue("example.io/fact", "value"))
	g.Expect(cloned).ShouldNot(BeIdenticalTo(profile))
}
