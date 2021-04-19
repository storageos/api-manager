package cache

import (
	"time"

	cache "github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Object is a cache to store object data.
type Object struct {
	*cache.Cache
}

// New initializes and returns a new object cache.
func New(expiryInterval, cleanupInterval time.Duration) *Object {
	return &Object{
		Cache: cache.New(expiryInterval, cleanupInterval),
	}
}

// CacheMiss implements external controller Cache interface.
// It checks for cache miss with the given object. On cache miss, it updates
// the cache.
func (c *Object) CacheMiss(obj client.Object) bool {
	key := client.ObjectKeyFromObject(obj).String()

	cached, found := c.Get(key)
	if found && equality.Semantic.DeepEqual(cached, obj) {
		return false
	}

	c.Set(key, obj, 0)
	return true
}
