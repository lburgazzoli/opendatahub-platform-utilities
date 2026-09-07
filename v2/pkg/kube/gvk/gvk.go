// Package gvk contains distribution-neutral Kubernetes GVK constants.
package gvk

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//nolint:gochecknoglobals // These are immutable package-level protocol values.
var (
	Namespace                      = corev1.SchemeGroupVersion.WithKind("Namespace")
	Service                        = corev1.SchemeGroupVersion.WithKind("Service")
	ServiceAccount                 = corev1.SchemeGroupVersion.WithKind("ServiceAccount")
	ConfigMap                      = corev1.SchemeGroupVersion.WithKind("ConfigMap")
	Secret                         = corev1.SchemeGroupVersion.WithKind("Secret")
	Role                           = rbacv1.SchemeGroupVersion.WithKind("Role")
	RoleBinding                    = rbacv1.SchemeGroupVersion.WithKind("RoleBinding")
	ClusterRole                    = rbacv1.SchemeGroupVersion.WithKind("ClusterRole")
	ClusterRoleBinding             = rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding")
	Deployment                     = appsv1.SchemeGroupVersion.WithKind("Deployment")
	StatefulSet                    = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
	DaemonSet                      = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
	Job                            = batchv1.SchemeGroupVersion.WithKind("Job")
	CronJob                        = batchv1.SchemeGroupVersion.WithKind("CronJob")
	MutatingWebhookConfiguration   = admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingWebhookConfiguration")
	ValidatingWebhookConfiguration = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingWebhookConfiguration")
	CustomResourceDefinition       = extensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")
	Lease                          = coordinationv1.SchemeGroupVersion.WithKind("Lease")
	MonitoringStack                = schema.GroupVersionKind{Group: "monitoring.rhobs", Version: "v1alpha1", Kind: "MonitoringStack"}
	TempoMonolithic                = schema.GroupVersionKind{Group: "tempo.grafana.com", Version: "v1alpha1", Kind: "TempoMonolithic"}
	TempoStack                     = schema.GroupVersionKind{Group: "tempo.grafana.com", Version: "v1alpha1", Kind: "TempoStack"}
	OpenTelemetryCollector         = schema.GroupVersionKind{Group: "opentelemetry.io", Version: "v1beta1", Kind: "OpenTelemetryCollector"}
)
