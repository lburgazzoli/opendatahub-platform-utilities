package singleton_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/admission/singleton"
)

type readerMock struct {
	mock.Mock
}

func (r *readerMock) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	options ...client.GetOption,
) error {
	return r.Called(ctx, key, object, options).Error(0)
}

func (r *readerMock) List(
	ctx context.Context,
	list client.ObjectList,
	options ...client.ListOption,
) error {
	return r.Called(ctx, list, options).Error(0)
}

func TestValidateCreationAllowsOnlyWhenNoObjectExists(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())

	gvk := corev1.SchemeGroupVersion.WithKind("ConfigMap")
	request := &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Create}}

	emptyClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	allowed := singleton.ValidateCreation(t.Context(), emptyClient, request, gvk)
	g.Expect(allowed.Allowed).Should(BeTrue())

	existingClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "existing"}},
	).Build()
	denied := singleton.ValidateCreation(t.Context(), existingClient, request, gvk)
	g.Expect(denied.Allowed).Should(BeFalse())
	g.Expect(denied.Result.Message).Should(ContainSubstring("only one instance"))
}

func TestValidateCreationSkipsNonCreateRequests(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "existing"}},
	).Build()
	request := &admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Update}}

	response := singleton.ValidateCreation(
		t.Context(),
		client,
		request,
		schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
	)
	g.Expect(response.Allowed).Should(BeTrue())
}

func TestCountTreatsMissingResourcesAsEmpty(t *testing.T) {
	t.Parallel()

	tests := map[string]error{
		"not found": apierrors.NewNotFound(schema.GroupResource{Group: "example.io", Resource: "widgets"}, "widgets"),
		"no match":  &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "example.io", Kind: "Widget"}},
	}

	for name, listError := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			reader := &readerMock{}
			reader.On("List", mock.Anything, mock.Anything, mock.Anything).Return(listError).Once()

			count, err := singleton.Count(
				t.Context(),
				reader,
				schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"},
			)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(count).Should(BeZero())
			reader.AssertExpectations(t)
		})
	}
}
