package annotations

const (
	// ManagedByODHOperator is the managed-resource opt-out convention. Presence
	// of this annotation means a resource must not be updated or garbage-collected
	// after initial creation.
	ManagedByODHOperator = "opendatahub.io/managed"

	InstanceName       = "platform.opendatahub.io/instance.name"
	InstanceNamespace  = "platform.opendatahub.io/instance.namespace"
	InstanceUID        = "platform.opendatahub.io/instance.uid"
	InstanceGeneration = "platform.opendatahub.io/instance.generation"
)
