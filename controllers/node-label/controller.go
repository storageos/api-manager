package nodelabel

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/annotations"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Reconciler reconciles a Node object by updating the StorageOS node object to
// match, or to remove it when deleted.
type Reconciler struct {
	client.Client
	Log      logr.Logger
	api      storageos.NodeLabeller
	recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewReconciler returns a new Node label reconciler.
func NewReconciler(api storageos.NodeLabeller, k8s client.Client, recorder record.EventRecorder) *Reconciler {
	return &Reconciler{
		Client:   k8s,
		Log:      ctrl.Log.WithName("controllers").WithName("NodeLabel"),
		api:      api,
		recorder: recorder,
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

// Reconcile applies labels set on the k8s node to the StorageOS node.
//
// StorageOS reserved labels are validated and applied first, then the remaining
// unreserved labels are applied.
//
// Any errors will result in a requeue, with standard back-off retries.
//
// There is no label sync from StorageOS to Kubernetes.  This is intentional to
// ensure a simple flow of desired state set by users in kubernetes to actual
// state set on the StorageOS node.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	node := &corev1.Node{}
	ctx := context.Background()
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		r.Log.Error(err, "unable to fetch Node")
		// Ignore not-found errors since they can't be fixed by an immediate
		// requeue.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.api.EnsureNodeLabels(node.Name, node.GetLabels()); err != nil {
		// Re-queue without error.  We will get frequent transient errors, such
		// as version conflicts or locked objects - that's ok, it will
		// eventually succeed.
		r.Log.Error(err, "failed to apply labels", "node", req.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	// Write an event related to the node object.
	r.recorder.Event(node, "Normal", "LabelsSynced", "synced labels to storageos")

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
func (c ChangePredicate) Update(e event.UpdateEvent) bool {
	// Ignore objects without metadata.
	if e.MetaOld == nil || e.MetaNew == nil {
		return false
	}

	// Ignore nodes that haven't run StorageOS.
	found, err := annotations.IncludesStorageOSDriver(e.MetaNew.GetAnnotations())
	if err != nil {
		c.log.Error(err, "failed to process node annotations", "node", e.MetaNew.GetName())
	}
	if !found {
		return false
	}

	// Reconcile only on label changes.
	if !reflect.DeepEqual(e.MetaOld.GetLabels(), e.MetaNew.GetLabels()) {
		return true
	}
	return false
}

// Delete determines whether an object delete should trigger a reconcile.
func (c ChangePredicate) Delete(e event.DeleteEvent) bool {
	return false
}

// Generic determines whether an generic event should trigger a reconcile.
func (c ChangePredicate) Generic(event.GenericEvent) bool {
	return false
}
