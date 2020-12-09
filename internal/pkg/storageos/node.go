package storageos

import (
	"context"
	"errors"
	"time"

	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	api "github.com/storageos/go-api/v2"
)

var (
	// ErrNodeNotFound is returned if a node was provided but it was not found.
	ErrNodeNotFound = errors.New("node not found")
)

// NodeLabeller provides access to update node labels.
type NodeLabeller interface {
	SetNodeLabels(name string, labels map[string]string) error
}

//NodeDeleter provides access to removing nodes from StorageOS.
//go:generate mockgen -destination=mocks/mock_node_deleter.go -package=mocks . NodeDeleter
type NodeDeleter interface {
	DeleteNode(name string) error
}

// GetNodeLabels retrieves the set of labels from a StorageOS node.
func (c *Client) GetNodeLabels(name string) (map[string]string, error) {
	funcName := "get_node_labels"
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
		return nil, observeErr(err)
	}
	return node.Labels, observeErr(nil)
}

// SetNodeLabels applies a set of labels to the StorageOS node.  Existing labels
// will be overwritten.
func (c *Client) SetNodeLabels(name string, labels map[string]string) error {
	funcName := "set_node_labels"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, GetAPIErrorRootCause(e))
		return e
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	ctx, cancel := context.WithTimeout(c.ctx, DefaultRequestTimeout)
	defer cancel()

	node, err := c.getNodeByName(ctx, name)
	if err != nil {
		return observeErr(err)
	}

	if _, _, err = c.api.DefaultApi.UpdateNode(ctx, node.Id, api.UpdateNodeData{Labels: labels, Version: node.Version}); err != nil {
		return observeErr(err)
	}
	return observeErr(nil)
}

// DeleteNode removes a node from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met (i.e. no active volumes).
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

	if _, err = c.api.DeleteNode(ctx, node.Id, node.Version, nil); err != nil {
		return observeErr(err)
	}
	return observeErr(nil)
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
