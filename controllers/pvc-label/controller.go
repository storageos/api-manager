package pvclabel

import (
	"context"
	"reflect"

	msyncv1 "github.com/darkowlzz/operator-toolkit/controller/metadata-sync/v1"
	"github.com/darkowlzz/operator-toolkit/object"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// Controller implements the Sync contoller interface, applying PVC labels to
// StorageOS volumes.
type Controller struct {
	api    VolumeLabeller
	scheme *runtime.Scheme
	log    logr.Logger
}

var _ msyncv1.Controller = &Controller{}

// NewController returns a Controller that implements PVC label sync in
// StorageOS.
func NewController(api VolumeLabeller, scheme *runtime.Scheme, log logr.Logger) (*Controller, error) {
	return &Controller{api: api, scheme: scheme, log: log}, nil
}

// Ensure applies labels set on the k8s PVC to the StorageOS volume.
//
// StorageOS reserved labels are validated and applied first, then the remaining
// unreserved labels are applied.
//
// Any errors will result in a requeue, with standard back-off retries.
//
// There is no label sync from StorageOS to Kubernetes.  This is intentional to
// ensure a simple flow of desired state set by users in Kubernetes to actual
// state set on the StorageOS volume.
func (c Controller) Ensure(ctx context.Context, obj client.Object) error {
	ctx, cancel := context.WithTimeout(ctx, storageos.DefaultRequestTimeout)
	defer cancel()

	// The PV name is required, as this will be the name of the StorageOS
	// volume.  We can get this without re-fetching the PVC by converting to an
	// unstructured object, and then reading from the spec.
	u, err := object.GetUnstructuredObject(c.scheme, obj)
	if err != nil {
		return err
	}
	pvName, ok, err := unstructured.NestedString(u.Object, []string{"spec", "volumeName"}...)
	if err != nil {
		return errors.Wrap(err, "failed to get pv name from pvc")
	}
	if !ok {
		return errors.Wrap(err, "pv for pvc not yet provisioned")
	}

	// Use the PV name, and the PVC namespace for the StorageOS volume lookup.
	key := client.ObjectKey{Name: pvName, Namespace: obj.GetNamespace()}
	if err := c.api.EnsureVolumeLabels(ctx, key, obj.GetLabels()); err != nil {
		return errors.Wrap(err, "requeuing operation")
	}
	c.log.Info("pvc labels applied to storageos", "name", obj.GetName())
	return nil
}

// Diff takes a list of Kubernets PVC objects and returns them if they exist
// as volumes within StorageOS but the labels are different.
func (c Controller) Diff(ctx context.Context, objs []client.Object) ([]client.Object, error) {
	ctx, cancel := context.WithTimeout(ctx, storageos.DefaultRequestTimeout)
	defer cancel()

	var apply []client.Object

	volumes, err := c.api.VolumeObjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		vol, ok := volumes[client.ObjectKeyFromObject(obj)]
		if !ok {
			// Ignore PVCs without volumes in StorageOS.
			continue
		}
		// If labels don't match, return original object.
		if !reflect.DeepEqual(obj.GetLabels(), vol.GetLabels()) {
			apply = append(apply, obj)
		}
	}
	return apply, nil
}

// Delete is a no-op.  Volume removal is handled via CSI.
func (c Controller) Delete(ctx context.Context, obj client.Object) error {
	return nil
}
