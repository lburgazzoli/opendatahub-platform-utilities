package dynamicwatcher

import (
	"errors"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type mockController struct {
	mock.Mock
	controller.Controller
}

func (c *mockController) Watch(value source.TypedSource[reconcile.Request]) error {
	return c.Called(value).Error(0)
}

var errWatchRegistration = errors.New("watch registration failed") //nolint:err113 // Test-only sentinel.
