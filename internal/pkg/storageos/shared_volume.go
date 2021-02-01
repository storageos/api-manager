package storageos

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/storageos/api-manager/internal/pkg/endpoint"
	"github.com/storageos/api-manager/internal/pkg/storageos/metrics"

	api "github.com/storageos/go-api/v2"
)

var (
	// ErrNotFound is returned if a volume was provided but it was not found.
	ErrNotFound = errors.New("volume not found")

	// ErrNotShared is returned if a volume was found but it is not shared.
	ErrNotShared = errors.New("volume not shared")

	// ErrNotKubernetes is returned if a volume was not created by the
	// Kubernetes volume provisioner.  This is required as the provisioner adds
	// labels to the volume that allows it to be traced back to the PVC.
	// Without the link to the PVC, we can't tell if the volume was created as
	// RWX and we can set the PVC as the OwnerReference, allowing cleanup on
	// PVC delete.
	ErrNotKubernetes = errors.New("volume not created by kubernetes")

	// ErrVolumeShared can be returned when the volume is expected not to be shared.
	ErrVolumeShared = errors.New("volume is shared")

	// ErrListingVolumes can be returned if there was an error listing volumes.
	ErrListingVolumes = errors.New("failed to list volumes")
)

// VolumeSharer provides access to StorageOS SharedVolumes.
//go:generate mockgen -destination=mocks/mock_volume_sharer.go -package=mocks . VolumeSharer
type VolumeSharer interface {
	ListSharedVolumes(ctx context.Context) (SharedVolumeList, error)
	SetExternalEndpoint(ctx context.Context, id string, namespace string, endpoint string) error
}

// ListSharedVolumes returns a list of active shared volumes.
func (c *Client) ListSharedVolumes(ctx context.Context) (SharedVolumeList, error) {
	funcName := "list_shared_volumes"
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

	var errors *multierror.Error
	var sharedVolumes SharedVolumeList

	for _, ns := range namespaces {
		volumes, resp, err := c.api.ListVolumes(ctx, ns.Id)
		if err != nil {
			errors = multierror.Append(errors, observeErr(api.MapAPIError(err, resp)))
		}

		for _, vol := range volumes {
			// Ignore volumes that are not shared or that have incorrectly
			// formatted endpoints.
			sv, err := toSharedVolume(ns.Name, vol)
			if err == nil {
				sharedVolumes = append(sharedVolumes, sv)
			}
		}
		// Bail on errors.
		if errors != nil {
			return nil, errors.ErrorOrNil()
		}
	}
	return sharedVolumes, observeErr(err)
}

func toSharedVolume(namespace string, vol api.Volume) (*SharedVolume, error) {
	// Skip non-k8s volumes.  The PV name & PVC namespace will be used as the
	// Service and Endpoints name & namespace.  The PVC name is required to set
	// the Service ownerRef to the PVC.
	pvName := vol.Labels[LabelPVName]
	pvcName := vol.Labels[LabelPVCName]
	pvcNamespace := vol.Labels[LabelPVCNamespace]
	if pvName == "" || pvcName == "" || pvcNamespace == "" {
		return nil, ErrNotKubernetes
	}

	// Skip volumes that don't have a valid NFS Endpoint.
	if vol.Nfs.ServiceEndpoint == nil {
		return nil, ErrNotShared
	}
	_, _, err := endpoint.SplitAddressPort(*vol.Nfs.ServiceEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "invalid internal endpoint")
	}

	// External service address, if any.
	extEndpoint := vol.Labels[LabelNFSMountEndpoint]

	return &SharedVolume{
		ID:               vol.Id,
		ServiceName:      pvName,
		PVCName:          pvcName,
		Namespace:        pvcNamespace,
		InternalEndpoint: *vol.Nfs.ServiceEndpoint,
		ExternalEndpoint: extEndpoint,
	}, nil
}

// SetExternalEndpoint sets the external endpoint on a SharedVolume.  The
// endpoint should be <host|ip>:<port>.
func (c *Client) SetExternalEndpoint(ctx context.Context, id string, namespace string, endpoint string) error {
	funcName := "set_external_endpoint"
	start := time.Now()
	defer func() {
		metrics.Latency.Observe(funcName, time.Since(start))
	}()
	observeErr := func(e error) error {
		metrics.Errors.Increment(funcName, e)
		return e
	}

	ctx = c.AddToken(ctx)

	curVol, err := c.getVolume(ctx, id, namespace)
	if err != nil {
		return observeErr(err)
	}

	mountEndpoint := curVol.Labels[LabelNFSMountEndpoint]
	if mountEndpoint == endpoint {
		metrics.Errors.Increment(funcName, nil)
		return nil
	}

	if resp, err := c.api.UpdateNFSVolumeMountEndpoint(ctx, curVol.NamespaceID, curVol.Id, api.NfsVolumeMountEndpoint{MountEndpoint: endpoint, Version: curVol.Version}, nil); err != nil {
		return observeErr(api.MapAPIError(err, resp))
	}
	return observeErr(nil)
}

// TODO: Merge with getVolumeByKey() and accept a key.
func (c *Client) getVolume(ctx context.Context, id string, namespace string) (*api.Volume, error) {
	ns, err := c.getNamespace(ctx, namespace)

	if err != nil {
		return nil, err
	}
	vol, resp, err := c.api.GetVolume(ctx, ns.Id, id)
	return &vol, api.MapAPIError(err, resp)
}

// TODO: Merge with getNamespaceByName() and accept a key.
func (c *Client) getNamespace(ctx context.Context, namespace string) (*api.Namespace, error) {
	namespaces, resp, err := c.api.ListNamespaces(ctx)
	if err != nil {
		return nil, api.MapAPIError(err, resp)
	}
	for _, ns := range namespaces {
		if ns.Name == namespace {
			return &ns, nil
		}
	}
	return nil, ErrNamespaceNotFound
}
