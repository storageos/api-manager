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
	DeleteNode(name string) error
	ListNodes() ([]types.NamespacedName, error)
}

// ListNodes returns a list of all StorageOS node objects.
func (c *Client) ListNodes() ([]types.NamespacedName, error) {
	funcName := "list_nodes"
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

	nodes, _, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, observeErr(err)
	}
	nn := []types.NamespacedName{}
	for _, node := range nodes {
		nn = append(nn, types.NamespacedName{Name: node.Name})
	}
	return nn, nil
}

// DeleteNode removes a node from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met:
// 		(1) The node appears offline
// 		(2) No node lock is held
//      (3) No master deployments live on the node (i.e. node must be detected as no active volumes).
func (c *Client) DeleteNode(name string) error {
	funcName := "delete_node"
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

	node, err := c.getNodeByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	resp, err := c.api.DeleteNode(ctx, node.Id, node.Version, nil)
	if err != nil {
		err = observeErr(err)

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
	nodes, _, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if node.Name == name {
			return &node, nil
		}
	}
	return nil, ErrNodeNotFound
}
