package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const defaultCacheTTL = 10 * time.Minute

type cacheEntry struct {
	created time.Time
}

//nolint:govet // mutex and TTL are intentionally adjacent to the cache map.
type Cache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

// NewCache creates a process-local deploy cache from optional settings.
func NewCache(options *CacheOptions) *Cache {
	if options == nil {
		return nil
	}

	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	return &Cache{ttl: ttl, entries: make(map[string]cacheEntry)}
}

func (cache *Cache) Sync() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	now := time.Now()
	for key, entry := range cache.entries {
		if now.Sub(entry.created) >= cache.ttl {
			delete(cache.entries, key)
		}
	}
}

func (cache *Cache) Add(
	deployed *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error {
	if deployed == nil || desired == nil {
		return nil
	}

	key, err := cache.key(deployed, desired)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	cache.entries[key] = cacheEntry{created: time.Now()}
	cache.mu.Unlock()

	return nil
}

func (cache *Cache) Has(
	deployed *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) (bool, error) {
	if deployed == nil || desired == nil {
		return false, nil
	}

	key, err := cache.key(deployed, desired)
	if err != nil {
		return false, err
	}

	cache.mu.RLock()
	entry, found := cache.entries[key]
	if !found {
		cache.mu.RUnlock()

		return false, nil
	}
	if time.Since(entry.created) < cache.ttl {
		cache.mu.RUnlock()

		return true, nil
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if current, ok := cache.entries[key]; ok && time.Since(current.created) >= cache.ttl {
		delete(cache.entries, key)
	}

	return false, nil
}

func (cache *Cache) Delete(
	deployed *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error {
	if deployed == nil || desired == nil {
		return nil
	}

	key, err := cache.key(deployed, desired)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	delete(cache.entries, key)
	cache.mu.Unlock()

	return nil
}

func (cache *Cache) key(
	deployed *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) (string, error) {
	data, err := json.Marshal(desired.Object)
	if err != nil {
		return "", fmt.Errorf("hash desired object: %w", err)
	}
	hash := sha256.Sum256(data)

	return fmt.Sprintf("%s/%s/%s/%s/%s/%s",
		deployed.GroupVersionKind(),
		deployed.GetNamespace(),
		deployed.GetName(),
		deployed.GetResourceVersion(),
		hex.EncodeToString(hash[:]), desired.GetUID(),
	), nil
}
