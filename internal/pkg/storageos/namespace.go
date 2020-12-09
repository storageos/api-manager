package storageos

import (
	"context"
	"errors"
	"time"

	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
)

var (
	// ErrNamespaceNotFound is returned if a namespace was provided but it was
	// not found.
	ErrNamespaceNotFound = errors.New("namespace not found")
)

//NamespaceDeleter provides access to removing namespaces from StorageOS.
type NamespaceDeleter interface {
	DeleteNamespace(name string) error
}

// DeleteNamespace removes a namespace from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met (i.e. namespace has volumes).
func (c *Client) DeleteNamespace(name string) error {
	funcName := "delete_namespace"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, GetAPIErrorRootCause(e))
		return e
	}

	ctx, cancel := context.WithTimeout(c.ctx, DefaultRequestTimeout)
	defer cancel()

	ns, err := c.getNamespaceByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	if _, err = c.api.DefaultApi.DeleteNamespace(ctx, ns.Id, ns.Version, nil); err != nil {
		return observeErr(err)
	}
	return observeErr(nil)
}

// getNamespaceByName returns the StorageOS namespace object matching the name, if any.
func (c *Client) getNamespaceByName(ctx context.Context, name string) (*api.Namespace, error) {
	namespaces, _, err := c.api.DefaultApi.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, ns := range namespaces {
		if ns.Name == name {
			return &ns, nil
		}
	}
	return nil, ErrNamespaceNotFound
}
