package nodelabel

import (
	"context"

	syncv1 "github.com/darkowlzz/operator-toolkit/controller/sync/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// Controller implements the Sync contoller interface, applying node labels to
// StorageOS nodes.
type Controller struct {
	api storageos.NodeLabeller
	log logr.Logger
}

var _ syncv1.Controller = &Controller{}

// NewController returns a Controller that implements node label sync in
// StorageOS.
func NewController(api storageos.NodeLabeller, log logr.Logger) (*Controller, error) {
	return &Controller{api: api, log: log}, nil
}

// Ensure applies labels set on the k8s node to the StorageOS node.
//
// StorageOS reserved labels are validated and applied first, then the remaining
// unreserved labels are applied.
//
// Any errors will result in a requeue, with standard back-off retries.
//
// There is no label sync from StorageOS to Kubernetes.  This is intentional to
// ensure a simple flow of desired state set by users in Kubernetes to actual
// state set on the StorageOS node.
func (c Controller) Ensure(ctx context.Context, obj client.Object) error {
	if err := c.api.EnsureNodeLabels(obj.GetName(), obj.GetLabels()); err != nil {
		return errors.Wrap(err, "requeuing operation")
	}
	c.log.Info("node labels applied to storageos", "name", obj.GetName())
	return nil
}

// Delete is a no-op.  The node-delete controller will handle deletes.
func (c Controller) Delete(ctx context.Context, obj client.Object) error {
	return nil
}

// List is a no-op.  The reconcile performs its own resync.
func (c Controller) List(ctx context.Context) ([]types.NamespacedName, error) {
	return nil, nil
}
