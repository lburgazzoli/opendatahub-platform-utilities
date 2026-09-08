package manager

import "sigs.k8s.io/controller-runtime/pkg/cache"

func cacheOptions(namespace string) cache.Options {
	return cache.Options{
		ReaderFailOnMissingInformer: true,
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
	}
}
