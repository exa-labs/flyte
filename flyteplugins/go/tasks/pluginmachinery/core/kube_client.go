package core

import (
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO we may not want to expose this?
// A friendly controller-runtime client that gets passed to executors
type KubeClient interface {
	// GetClient returns a client configured with the Config
	GetClient() client.Client

	// GetCache returns a cache.Cache
	GetCache() cache.Cache

	// GetAPIReader returns a reader that bypasses the local informer cache and
	// reads directly from the API server. Use this on paths where a stale cache
	// could cause silent correctness bugs (e.g. clearing finalizers).
	GetAPIReader() client.Reader
}
