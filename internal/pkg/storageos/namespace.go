package storageos

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
	"k8s.io/apimachinery/pkg/types"
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
	DeleteNamespace(ctx context.Context, name string) error
	ListNamespaces(ctx context.Context) ([]types.NamespacedName, error)
}

// ListNamespaces returns a list of all StorageOS namespace objects.
func (c *Client) ListNamespaces(ctx context.Context) ([]types.NamespacedName, error) {
	funcName := "list_namespaces"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, GetAPIErrorRootCause(e))
		return e
	}

	ctx = c.AddToken(ctx)

	namespaces, _, err := c.api.ListNamespaces(ctx)
	if err != nil {
		return nil, observeErr(err)
	}
	nn := []types.NamespacedName{}
	for _, ns := range namespaces {
		nn = append(nn, types.NamespacedName{Name: ns.Name})
	}
	return nn, nil
}

// DeleteNamespace removes a namespace from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met (i.e. namespace has volumes).
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	funcName := "delete_namespace"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, GetAPIErrorRootCause(e))
		return e
	}

	ctx = c.AddToken(ctx)

	ns, err := c.getNamespaceByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	resp, err := c.api.DeleteNamespace(ctx, ns.Id, ns.Version, nil)
	if err != nil {
		err = observeErr(err)

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

// getNamespaceByName returns the StorageOS namespace object matching the name, if any.
func (c *Client) getNamespaceByName(ctx context.Context, name string) (*api.Namespace, error) {
	namespaces, _, err := c.api.ListNamespaces(ctx)
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
