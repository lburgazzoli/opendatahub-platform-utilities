package metadata_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	platformlabels "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
)

func TestDefaultPolicyAppliesAndMatchesOwnerMetadata(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	owner := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "namespace", UID: "uid", Generation: 3,
	}}
	owner.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Module"})

	object := &metav1.PartialObjectMetadata{}
	policy := metadata.DefaultPolicy()

	g.Expect(policy.Apply(object, owner)).Should(Succeed())
	g.Expect(object.GetLabels()).Should(HaveKeyWithValue(platformlabels.PlatformPartOf, "module"))
	g.Expect(object.GetAnnotations()).Should(HaveKeyWithValue(platformannotations.InstanceGeneration, "3"))
	g.Expect(policy.Matches(object, owner)).Should(BeTrue())
	g.Expect(policy.Selector(owner).Matches(labels.Set(object.GetLabels()))).Should(BeTrue())

	object.SetAnnotations(map[string]string{platformannotations.InstanceGeneration: "2"})
	g.Expect(policy.Matches(object, owner)).Should(BeFalse())
}

func TestComposedPolicyIntersectsSelectorsAndVerifiesMatches(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	owner := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "owner", UID: "uid"}}
	owner.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Module"})

	object := &metav1.PartialObjectMetadata{}
	policy := metadata.Compose(metadata.DefaultPolicy(), testPolicy{})

	g.Expect(policy.Apply(object, owner)).Should(Succeed())
	g.Expect(policy.Matches(object, owner)).Should(BeTrue())
	g.Expect(policy.Selector(owner).Matches(labels.Set(object.GetLabels()))).Should(BeTrue())
}

type testPolicy struct{}

func (testPolicy) Apply(object, _ client.Object) error {
	labels := object.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels["example.io/policy"] = "true"
	object.SetLabels(labels)

	return nil
}

func (testPolicy) Selector(_ client.Object) labels.Selector {
	return labels.SelectorFromSet(labels.Set{"example.io/policy": "true"})
}

func (testPolicy) Matches(object, _ client.Object) bool {
	return object.GetLabels()["example.io/policy"] == "true"
}
