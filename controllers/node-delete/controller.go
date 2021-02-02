package nodedelete

import (
	"context"

	syncv1 "github.com/darkowlzz/operator-toolkit/controller/sync/v1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// Controller implements the SyncReconciler contoller interface, deleting nodes
// in StorageOS when they have been detected as deleted in Kubernetes.
type Controller struct {
	api storageos.NodeDeleter
	log logr.Logger
}

var _ syncv1.Controller = &Controller{}

// NewController returns a Controller that implements node garbage collection in
// StorageOS.
func NewController(api storageos.NodeDeleter, log logr.Logger) (*Controller, error) {
	return &Controller{api: api, log: log}, nil
}

// Ensure is a no-op.  We only care about deletes.
func (c Controller) Ensure(ctx context.Context, obj client.Object) error {
	return nil
}

// Delete receives a k8s object that's been deleted and calls the StorageOS api
// to remove it from management.
func (c Controller) Delete(ctx context.Context, obj client.Object) error {
	ctx, cancel := context.WithTimeout(ctx, storageos.DefaultRequestTimeout)
	defer cancel()

	err := c.api.DeleteNode(ctx, client.ObjectKeyFromObject(obj))
	if err != nil && err != storageos.ErrNodeNotFound {
		return err
	}
	c.log.Info("node decommissioned in storageos", "name", obj.GetName())
	return nil
}

// List returns a list of nodes known to StorageOS, as NamespacedNames. This is
// used for garbage collection and can be expensive. The garbage collector is
// run in a separate goroutine periodically, not affecting the main
// reconciliation control-loop.
func (c Controller) List(ctx context.Context) ([]types.NamespacedName, error) {
	ctx, cancel := context.WithTimeout(ctx, storageos.DefaultRequestTimeout)
	defer cancel()

	objects, err := c.api.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	return storageos.ObjectKeys(objects), nil
}
