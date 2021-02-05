package storageos

import (
	"context"
	"errors"
	"net/http"
	"time"

	api "github.com/storageos/go-api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storageosv1 "github.com/storageos/api-manager/api/v1"
	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
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

// NodeObjects returns a map of node objects, indexed on ObjectKey for efficient
// lookups.
func (c *Client) NodeObjects(ctx context.Context) (map[client.ObjectKey]Object, error) {
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
	objects := make(map[client.ObjectKey]Object)
	for _, node := range nodes {
		objects[client.ObjectKey{Name: node.GetName()}] = node
	}

	return objects, nil
}

// ListNodes returns a list of all StorageOS node objects.
func (c *Client) ListNodes(ctx context.Context) ([]client.Object, error) {
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
	objects := []client.Object{}
	for _, node := range nodes {
		objects = append(objects, nodeToCR(node))
	}

	return objects, nil
}

func nodeToCR(n api.Node) *storageosv1.Node {
	var health storageosv1.NodeHealth
	switch n.Health {
	case api.NODEHEALTH_ONLINE:
		health = storageosv1.NodeHealthOnline
	case api.NODEHEALTH_OFFLINE:
		health = storageosv1.NodeHealthOffline
	default:
		health = storageosv1.NodeHealthUnknown
	}
	return &storageosv1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: storageosv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              n.Name,
			UID:               types.UID(n.Id),
			ResourceVersion:   n.Version,
			CreationTimestamp: metav1.Time{Time: n.CreatedAt},
			Labels:            n.Labels,
		},
		Spec: storageosv1.NodeSpec{
			IoEndpoint:         n.IoEndpoint,
			SupervisorEndpoint: n.SupervisorEndpoint,
			GossipEndpoint:     n.GossipEndpoint,
			ClusteringEndpoint: n.ClusteringEndpoint,
		},
		Status: storageosv1.NodeStatus{
			Health: health,
			Capacity: storageosv1.CapacityStats{
				Total:     n.Capacity.Total,
				Free:      n.Capacity.Free,
				Available: n.Capacity.Available,
			},
		},
	}
}

// DeleteNode removes a node from the StorageOS cluster.  Delete will fail if
// pre-requisites are not met:
// 		(1) The node appears offline
// 		(2) No node lock is held
//      (3) No master deployments live on the node (i.e. node must be detected as no active volumes).
func (c *Client) DeleteNode(ctx context.Context, key client.ObjectKey) error {
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

	node, err := c.getNodeByKey(ctx, key)
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

// getNodeByKey returns the StorageOS node object matching the name in the key, if any.
func (c *Client) getNodeByKey(ctx context.Context, key client.ObjectKey) (*api.Node, error) {
	nodes, resp, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, api.MapAPIError(err, resp)
	}
	for _, node := range nodes {
		if node.Name == key.Name {
			return &node, nil
		}
	}
	return nil, ErrNodeNotFound
}
