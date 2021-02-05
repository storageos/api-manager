package storageos

import (
	"context"
	"errors"
	"time"

	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"
	"github.com/storageos/go-api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrVolumeNotFound is returned if a volume was provided but it was not found.
	ErrVolumeNotFound = errors.New("volume not found")
)

// VolumeObjects returns a map of volume objects, indexed on object key for
// efficient lookups.
func (c *Client) VolumeObjects(ctx context.Context) (map[client.ObjectKey]Object, error) {
	funcName := "volume_objects"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	volumes, err := c.getVolumes(ctx)
	if err != nil {
		return nil, observeErr(err)
	}
	objects := make(map[client.ObjectKey]Object)
	for _, vol := range volumes {
		objects[ObjectKeyFromObject(vol)] = vol
	}

	return objects, nil
}

// ListVolumes returns a list of all StorageOS volume objects.
func (c *Client) ListVolumes(ctx context.Context) ([]Object, error) {
	funcName := "list_volumes"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	volumes, err := c.getVolumes(ctx)
	if err != nil {
		return nil, observeErr(err)
	}
	objects := []Object{}
	for _, vol := range volumes {
		objects = append(objects, vol)
	}

	return objects, nil
}

// getVolumes returns all StorageOS volumes.
func (c *Client) getVolumes(ctx context.Context) ([]api.Volume, error) {
	var volumes []api.Volume
	namespaces, err := c.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, ns := range namespaces {
		nsVols, resp, err := c.api.ListVolumes(ctx, ns.GetID())
		if err != nil {
			return nil, api.MapAPIError(err, resp)
		}
		volumes = append(volumes, nsVols...)
	}
	return volumes, nil
}

// GetVolume returns the StorageOS volume object matching the key.
func (c *Client) GetVolume(ctx context.Context, key client.ObjectKey) (Object, error) {
	funcName := "get_volume"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	vol, err := c.getVolume(ctx, key)
	if err != nil {
		return nil, observeErr(err)
	}
	return vol, nil
}

// GetVolume returns the StorageOS volume object matching the key.
func (c *Client) getVolume(ctx context.Context, key client.ObjectKey) (*api.Volume, error) {
	ns, err := c.getNamespace(ctx, client.ObjectKey{Name: key.Namespace})
	if err != nil {
		return nil, err
	}

	volumes, resp, err := c.api.ListVolumes(ctx, ns.Id)
	if err != nil {
		return nil, api.MapAPIError(err, resp)
	}
	for _, vol := range volumes {
		if vol.Name == key.Name {
			return &vol, nil
		}
	}
	return nil, ErrVolumeNotFound
}

// getVolumeByID returns a StorageOS volume using the Volume ID and Namespace
// name.  This is an internal optimisation for when the internal Volume ID is
// known.  Most callers should use getVolume() instead.
func (c *Client) getVolumeByID(ctx context.Context, id string, namespace string) (*api.Volume, error) {
	ns, err := c.getNamespace(ctx, client.ObjectKey{Name: namespace})
	if err != nil {
		return nil, err
	}
	vol, resp, err := c.api.GetVolume(ctx, ns.Id, id)
	return &vol, api.MapAPIError(err, resp)
}
