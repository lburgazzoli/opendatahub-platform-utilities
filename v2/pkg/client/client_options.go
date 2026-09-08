package client

import "sigs.k8s.io/controller-runtime/pkg/client"

// Option configures a cache-coherent client.
type Option func(*Client)

// New wraps a controller-runtime client for cache-coherent typed reads.
func New(inner client.Client, options ...Option) *Client {
	wrapped := &Client{inner: inner}
	for _, option := range options {
		if option != nil {
			option(wrapped)
		}
	}

	return wrapped
}
