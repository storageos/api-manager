package cache

import (
	"reflect"
	"time"

	"github.com/go-logr/logr"
	cache "github.com/patrickmn/go-cache"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Object is a cache to store object data.
type Object struct {
	*cache.Cache
	log logr.Logger
}

// Key returns the key in a cache, given name and namespace.
func Key(name, namespace string) string {
	return types.NamespacedName{Name: name, Namespace: namespace}.String()
}

// New initializes and returns a new object cache.
func New(expiryInterval, cleanupInterval time.Duration, log logr.Logger) *Object {
	return &Object{
		Cache: cache.New(expiryInterval, cleanupInterval),
		log:   log.WithName("ObjectCache"),
	}
}

// CacheMiss implements external controller Cache interface.
// It checks for cache miss with the given object. On cache miss, it updates
// the cache.
func (c *Object) CacheMiss(obj client.Object) bool {
	key := Key(obj.GetName(), obj.GetNamespace())

	cached, found := c.Get(key)
	if found && reflect.DeepEqual(cached, obj) {
		c.log.V(5).Info("cache hit", "name", obj.GetName(), "namespace", obj.GetNamespace())
		return false
	}

	c.Set(key, obj, 0)
	c.log.V(5).Info("cache miss", "name", obj.GetName(), "namespace", obj.GetNamespace())
	return true
}
