package deploy

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestDefaultFieldOwnerUsesOwnerKind(t *testing.T) {
	t.Parallel()

	owner := &corev1.ConfigMap{}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	g := NewWithT(t)
	g.Expect(defaultOptions().FieldOwner(owner)).Should(Equal("ConfigMap"))
}
