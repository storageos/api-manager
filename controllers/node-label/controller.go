package nodelabel

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run.
	MaxConcurrentReconciles = 10
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
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: MaxConcurrentReconciles}).
		For(&corev1.Node{}).
		WithEventFilter(ChangePredicate{}).
		Complete(r)
}

// Reconcile deletes the StorageOS node.  The delete will fail if the control
// plane has not yet determined that the node is offline.  Any errors will
// result in a requeue, with standard back-off retries.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	node := &corev1.Node{}
	ctx := context.Background()
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		r.Log.Error(err, "unable to fetch Node")
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get
		// them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.api.SetNodeLabels(node.Name, node.GetLabels()); err != nil && err != storageos.ErrNamespaceNotFound {
		// Re-queue without error.  We will get frequent transient errors, such
		// as version conflicts or locked objects - that's ok.
		return ctrl.Result{Requeue: true}, nil
	}

	// Write an event related to the node object.
	r.recorder.Event(node, "Normal", "LabelsSynced", "synced labels to storageos")

	return ctrl.Result{}, nil
}

// ChangePredicate filters events before enqueuing the keys.
type ChangePredicate struct {
	predicate.Funcs
}

// Create determines whether an object create should trigger a reconcile.
func (ChangePredicate) Create(event.CreateEvent) bool {
	return false
}

// Update determines whether an object update should trigger a reconcile.
func (ChangePredicate) Update(e event.UpdateEvent) bool {
	// Ignore objects without metadata.
	if e.MetaOld == nil || e.MetaNew == nil {
		return false
	}

	// Reconcile only on label changes.
	if !reflect.DeepEqual(e.MetaOld.GetLabels(), e.MetaNew.GetLabels()) {
		return true
	}
	return false
}

// Delete determines whether an object delete should trigger a reconcile.
func (ChangePredicate) Delete(e event.DeleteEvent) bool {
	return false
}

// Generic determines whether an generic event should trigger a reconcile.
func (ChangePredicate) Generic(event.GenericEvent) bool {
	return false
}
