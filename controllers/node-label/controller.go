package nodelabel

import (
	"context"
	"reflect"

	msyncv1 "github.com/darkowlzz/operator-toolkit/controller/metadata-sync/v1"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// Controller implements the Sync contoller interface, applying node labels to
// StorageOS nodes.
type Controller struct {
	api NodeLabeller
	log logr.Logger
}

var _ msyncv1.Controller = &Controller{}

// NewController returns a Controller that implements node label sync in
// StorageOS.
func NewController(api NodeLabeller, log logr.Logger) (*Controller, error) {
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
	tr := otel.Tracer("node-label")
	ctx, span := tr.Start(ctx, "node label ensure")
	span.SetAttributes(label.String("name", obj.GetName()))
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, storageos.DefaultRequestTimeout)
	defer cancel()

	if err := c.api.EnsureNodeLabels(ctx, client.ObjectKeyFromObject(obj), obj.GetLabels()); err != nil {
		span.RecordError(err)
		return err
	}
	span.SetStatus(codes.Ok, "node labels applied to storageos")
	c.log.Info("node labels applied to storageos", "name", obj.GetName())
	return nil
}

// Diff takes a list of Kubernets node objects and returns them if they exist
// within StorageOS but the labels are different.
func (c Controller) Diff(ctx context.Context, objs []client.Object) ([]client.Object, error) {
	tr := otel.Tracer("node-label")
	ctx, span := tr.Start(ctx, "node label diff")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, storageos.DefaultRequestTimeout)
	defer cancel()

	var apply []client.Object

	nodes, err := c.api.NodeObjects(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	for _, obj := range objs {
		node, ok := nodes[client.ObjectKeyFromObject(obj)]
		if !ok || node == nil {
			// Ignore nodes not already known to StorageOS.
			continue
		}
		// If labels don't match, return original object.
		if !reflect.DeepEqual(obj.GetLabels(), node.GetLabels()) {
			apply = append(apply, obj)
		}
	}
	span.SetAttributes(label.Int("stale nodes", len(apply)))
	span.SetStatus(codes.Ok, "compared nodes")
	return apply, nil
}

// Delete is a no-op.  The node-delete controller will handle deletes.
func (c Controller) Delete(ctx context.Context, obj client.Object) error {
	return nil
}
