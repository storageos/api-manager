package storageos

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
)

var (
	// ErrNodeNotFound is returned if a node was provided but it was not found.
	ErrNodeNotFound = errors.New("node not found")

	// ErrNodeInUse is returned when an operation can't be completed because
	// StorageOS detects that the node is still in use.
	ErrNodeInUse = errors.New("node still in use")

	// ErrNodeHasLock is returned when the node lock has not yet expired.
	ErrNodeHasLock = errors.New("node lock has not yet expired")
)

//NodeDeleter provides access to removing nodes from StorageOS.
//go:generate mockgen -destination=mocks/mock_node_deleter.go -package=mocks . NodeDeleter
type NodeDeleter interface {
	DeleteNode(ctx context.Context, name string) error
	ListNodes(ctx context.Context) ([]Object, error)
}

// NodeObjects returns a map of node objects, keyed on node name for efficient
// lookups.
func (c *Client) NodeObjects(ctx context.Context) (map[string]Object, error) {
	funcName := "node_objects"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	nodes, resp, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, observeErr(api.MapAPIError(err, resp))
	}
	objects := make(map[string]Object)
	for _, node := range nodes {
		objects[node.GetName()] = node
	}

	return objects, nil
}

// ListNodes returns a list of all StorageOS node objects.
func (c *Client) ListNodes(ctx context.Context) ([]Object, error) {
	funcName := "list_nodes"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	nodes, resp, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, observeErr(api.MapAPIError(err, resp))
	}
	objects := []Object{}
	for _, node := range nodes {
		objects = append(objects, node)
	}

	return objects, nil
}

// DeleteNode removes a node from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met:
// 		(1) The node appears offline
// 		(2) No node lock is held
//      (3) No master deployments live on the node (i.e. node must be detected as no active volumes).
func (c *Client) DeleteNode(ctx context.Context, name string) error {
	funcName := "delete_node"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	node, err := c.getNodeByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	resp, err := c.api.DeleteNode(ctx, node.Id, node.Version, nil)
	if err != nil {
		err = api.MapAPIError(err, resp)

		switch resp.StatusCode {
		case http.StatusConflict:
			return ErrNodeInUse
		case http.StatusLocked:
			return ErrNodeHasLock
		case http.StatusNotFound:
			return ErrNodeNotFound
		default:
			return err
		}
	}
	return nil
}

// getNodeByName returns the StorageOS node object matching the name, if any.
func (c *Client) getNodeByName(ctx context.Context, name string) (*api.Node, error) {
	nodes, resp, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, api.MapAPIError(err, resp)
	}
	for _, node := range nodes {
		if node.Name == name {
			return &node, nil
		}
	}
	return nil, ErrNodeNotFound
}
