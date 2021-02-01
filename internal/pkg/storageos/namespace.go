package storageos

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrNamespaceNotFound is returned if a namespace was provided but it was
	// not found.
	ErrNamespaceNotFound = errors.New("namespace not found")

	// ErrNamespaceInUse is returned when an operation can't be completed because
	// StorageOS detects that the namespace is still in use.
	ErrNamespaceInUse = errors.New("namespace still in use")
)

//NamespaceDeleter provides access to removing namespaces from StorageOS.
//go:generate mockgen -destination=mocks/mock_namespace_deleter.go -package=mocks . NamespaceDeleter
type NamespaceDeleter interface {
	DeleteNamespace(ctx context.Context, key client.ObjectKey) error
	ListNamespaces(ctx context.Context) ([]Object, error)
}

// ListNamespaces returns a list of all StorageOS namespace objects.
func (c *Client) ListNamespaces(ctx context.Context) ([]Object, error) {
	funcName := "list_namespaces"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	namespaces, resp, err := c.api.ListNamespaces(ctx)
	if err != nil {
		return nil, observeErr(api.MapAPIError(err, resp))
	}
	objects := []Object{}
	for _, ns := range namespaces {
		objects = append(objects, ns)
	}
	return objects, nil
}

// DeleteNamespace removes a namespace from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met (i.e. namespace has volumes).
func (c *Client) DeleteNamespace(ctx context.Context, key client.ObjectKey) error {
	funcName := "delete_namespace"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	ns, err := c.getNamespace(ctx, key)
	if err != nil {
		return observeErr(err)
	}

	resp, err := c.api.DeleteNamespace(ctx, ns.Id, ns.Version, nil)
	if err != nil {
		err = observeErr(api.MapAPIError(err, resp))

		switch resp.StatusCode {
		case http.StatusConflict:
			return ErrNamespaceInUse
		case http.StatusNotFound:
			return ErrNamespaceNotFound
		default:
			return err
		}
	}
	return nil
}

// getNamespace returns the StorageOS namespace object matching the key,
// if any.
func (c *Client) getNamespace(ctx context.Context, key client.ObjectKey) (*api.Namespace, error) {
	namespaces, resp, err := c.api.ListNamespaces(ctx)
	if err != nil {
		return nil, api.MapAPIError(err, resp)
	}
	for _, ns := range namespaces {
		if ns.Name == key.Name {
			return &ns, nil
		}
	}
	return nil, ErrNamespaceNotFound
}
