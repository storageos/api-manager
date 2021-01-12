package nodelabel

import (
	"context"
	"fmt"
	"time"

	syncv1 "github.com/darkowlzz/operator-toolkit/controller/sync/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// Reconciler reconciles a Node object by applying labels from the Kubernetes
// node to the StorageOS node object.
type Reconciler struct {
	client.Client
	log            logr.Logger
	api            storageos.NodeLabeller
	resyncInterval time.Duration

	syncv1.Reconciler
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewReconciler returns a new Node label reconciler.
//
// The resyncInterval determines how often the periodic resync operation should
// be run.
func NewReconciler(api storageos.NodeLabeller, k8s client.Client, resyncInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:         k8s,
		log:            ctrl.Log,
		api:            api,
		resyncInterval: resyncInterval,
	}
}

// SetupWithManager registers the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	c, err := NewController(r.api, r.log)
	if err != nil {
		return err
	}

	// Set the resync function & interval.
	sf := syncv1.NewSyncFunc(r.resync, r.resyncInterval)

	// Initialize the reconciler.
	err = r.Reconciler.Init(mgr, &corev1.Node{}, &corev1.NodeList{},
		syncv1.WithName("node-label-sync"),
		syncv1.WithController(c),
		syncv1.WithLogger(r.log),
		syncv1.WithSyncFuncs([]syncv1.SyncFunc{sf}),
	)
	if err != nil {
		return fmt.Errorf("failed to create new reconciler: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		For(&corev1.Node{}).
		WithEventFilter(Predicate{log: r.log}).
		Complete(r)
}

// resync runs periodically to resync node labels with the StorageOS node objects.
func (r *Reconciler) resync() {
	r.log.WithValues("resync", r.Name)

	ctx := context.Background()

	// List all the k8s node objects.
	instances := r.PrototypeList.DeepCopyObject().(client.ObjectList)
	if err := r.Client.List(ctx, instances); err != nil {
		r.log.Info("failed to list nodes", "error", err)
		return
	}
	items, err := apimeta.ExtractList(instances)
	if err != nil {
		r.log.Info("failed to extract node objects from list", "error", err)
		return
	}
	for _, item := range items {
		// Get meta object from the item and extract namespace/name info.
		obj, err := apimeta.Accessor(item)
		if err != nil {
			r.log.Info("failed to get accessor for node item", "item", item, "error", err)
			continue
		}
		if err := r.api.EnsureNodeLabels(obj.GetName(), obj.GetLabels()); err != nil {
			r.log.Info("failed to sync node labels", "node", obj.GetName(), "error", err)
		}
	}
}
