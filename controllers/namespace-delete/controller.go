package nsdelete

import (
	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Reconciler reconciles a Namespace object by deleting the StorageOS namespace
// when the corresponding Kubernetes namespace is deleted.
type Reconciler struct {
	client.Client
	Log logr.Logger
	api storageos.NamespaceDeleter
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// NewReconciler returns a new Namespace delete reconciler.
func NewReconciler(api storageos.NamespaceDeleter, k8s client.Client) *Reconciler {
	return &Reconciler{
		Client: k8s,
		Log:    ctrl.Log.WithName("controllers").WithName("NamespaceDelete"),
		api:    api,
	}
}

// SetupWithManager registers the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		For(&corev1.Namespace{}).
		WithEventFilter(ChangePredicate{}).
		Complete(r)
}

// Reconcile deletes the StorageOS namespace.  The delete will fail if there are
// still volumes in the namespace.  Any errors will result in a requeue, with
// standard back-off retries.
//
// Events are not sent as they require an object. By this point the namespace
// object will no longer be available.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	err := r.api.DeleteNamespace(req.Name)
	if err != nil && err != storageos.ErrNamespaceNotFound {
		// Re-queue without error.  We will get frequent transient errors, such
		// as version conflicts or locked objects - that's ok.  Even refusal to
		// delete due to remaining volumes should be seen as transient and
		// should not raise an error.
		return ctrl.Result{Requeue: true}, nil
	}
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
func (ChangePredicate) Update(event.UpdateEvent) bool {
	return false
}

// Delete determines whether an object delete should trigger a reconcile.
func (ChangePredicate) Delete(e event.DeleteEvent) bool {
	// Ignore objects without metadata.
	return e.Meta != nil
}

// Generic determines whether an generic event should trigger a reconcile.
func (ChangePredicate) Generic(event.GenericEvent) bool {
	return false
}
