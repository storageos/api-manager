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

// VolumeObjects returns a map of volume objects, keyed on volume name for efficient
// lookups.
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

// TODO: Merge with getVolume().
func (c *Client) getVolumeByKey(ctx context.Context, key client.ObjectKey) (*api.Volume, error) {
	ns, err := c.getNamespace(ctx, key.Namespace)
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
