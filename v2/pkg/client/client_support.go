package client

import "sigs.k8s.io/controller-runtime/pkg/client"

func hasFieldSelector(options []client.ListOption) bool {
	listOptions := &client.ListOptions{}
	for _, option := range options {
		option.ApplyToList(listOptions)
	}

	return listOptions.FieldSelector != nil
}
