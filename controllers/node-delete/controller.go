package nodedelete

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// DriverName is the name of the StorageOS CSI driver.
	DriverName = "csi.storageos.com"

	// DriverAnnotationKey is the annotation that stores the registered CSI
	// drivers.
	DriverAnnotationKey = "csi.volume.kubernetes.io/nodeid"
)

// Reconciler reconciles a Node object by deleting the StorageOS node object
// when the corresponding Kubernetes node is deleted.
type Reconciler struct {
	client.Client
	Log logr.Logger
	api storageos.NodeDeleter
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewReconciler returns a new Node delete reconciler.
func NewReconciler(api storageos.NodeDeleter, k8s client.Client) *Reconciler {
	return &Reconciler{
		Client: k8s,
		Log:    ctrl.Log.WithName("controllers").WithName("NodeDelete"),
		api:    api,
	}
}

// SetupWithManager registers the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		For(&corev1.Node{}).
		WithEventFilter(ChangePredicate{log: r.Log}).
		Complete(r)
}

// Reconcile deletes the StorageOS node.  The delete will fail if the control
// plane has not yet determined that the node is offline.  Any errors will
// result in a requeue, with standard back-off retries.
//
// Events are not sent as they require an object. By this point the namespace
// object will no longer be available.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	err := r.api.DeleteNode(req.Name)
	if err != nil && err != storageos.ErrNodeNotFound {
		// Re-queue without error.  We will get frequent transient errors, such
		// as version conflicts or locked objects - that's ok.  Even refusal to
		// delete due to the node still showing as online should be seen as
		// transient and should not raise an error.
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// ChangePredicate filters events before enqueuing the keys.
type ChangePredicate struct {
	predicate.Funcs
	log logr.Logger
}

// Create determines whether an object create should trigger a reconcile.
func (c ChangePredicate) Create(event.CreateEvent) bool {
	return false
}

// Update determines whether an object update should trigger a reconcile.
func (c ChangePredicate) Update(event.UpdateEvent) bool {
	return false
}

// Delete determines whether an object delete should trigger a reconcile.
func (c ChangePredicate) Delete(e event.DeleteEvent) bool {
	// Ignore objects without the StorageOS CSI driver annotation.
	found, err := hasStorageOSDriverAnnotation(e.Object.GetAnnotations())
	if err != nil {
		c.log.Error(err, "failed to process node annotations", "node", e.Object.GetName())
	}
	return found
}

// Generic determines whether an generic event should trigger a reconcile.
func (c ChangePredicate) Generic(event.GenericEvent) bool {
	return false
}

// hasStorageOSDriverAnnotation returns true if the node has the StorageOS
// driver annotation.  The annotation is added by the drivers' node registrar,
// so should only be present on nodes that have run StorageOS.
func hasStorageOSDriverAnnotation(annotations map[string]string) (bool, error) {
	drivers, ok := annotations[DriverAnnotationKey]
	if !ok {
		return false, nil
	}
	driversMap := map[string]string{}
	if err := json.Unmarshal([]byte(drivers), &driversMap); err != nil {
		return false, err
	}
	if _, ok := driversMap[DriverName]; ok {
		return true, nil
	}
	return false, nil
}
