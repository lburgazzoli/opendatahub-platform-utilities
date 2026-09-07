// Package gvk contains distribution-neutral Kubernetes GVK constants.
package gvk

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

//nolint:gochecknoglobals // These are immutable package-level protocol values.
var (
	Deployment     = appsv1.SchemeGroupVersion.WithKind("Deployment")
	Service        = corev1.SchemeGroupVersion.WithKind("Service")
	ServiceAccount = corev1.SchemeGroupVersion.WithKind("ServiceAccount")
	ConfigMap      = corev1.SchemeGroupVersion.WithKind("ConfigMap")
)
