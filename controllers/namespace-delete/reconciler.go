package nsdelete

import (
	"fmt"
	"time"

	objectv1 "github.com/darkowlzz/operator-toolkit/controller/external-object-sync/v1"
	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// Reconciler reconciles a Namespace object by deleting the StorageOS namespace
// when the corresponding Kubernetes namespace is deleted.
type Reconciler struct {
	client.Client
	log        logr.Logger
	api        storageos.NamespaceDeleter
	gcInterval time.Duration

	objectv1.SyncReconciler
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// NewReconciler returns a new Namespace delete reconciler.
//
// The gcInterval determines how often the periodic resync operation should be
// run.
func NewReconciler(api storageos.NamespaceDeleter, k8s client.Client, gcInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:     k8s,
		log:        ctrl.Log,
		api:        api,
		gcInterval: gcInterval,
	}
}

// SetupWithManager registers the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	c, err := NewController(r.api, r.log)
	if err != nil {
		return err
	}

	// Initialize the reconciler.
	err = r.SyncReconciler.Init(mgr, &corev1.Namespace{}, &corev1.NamespaceList{},
		objectv1.WithName("ns-delete"),
		objectv1.WithController(c),
		objectv1.WithGarbageCollectionPeriod(r.gcInterval),
		objectv1.WithLogger(r.log),
	)
	if err != nil {
		return fmt.Errorf("failed to create new SyncReconciler: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		For(&corev1.Namespace{}).
		WithEventFilter(Predicate{}).
		Complete(r)
}
