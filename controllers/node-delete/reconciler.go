package nodedelete

import (
	"context"
	"fmt"
	"time"

	objectv1 "github.com/darkowlzz/operator-toolkit/controller/external-object-sync/v1"
	syncv1 "github.com/darkowlzz/operator-toolkit/controller/sync/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

//NodeDeleter provides access to removing nodes from StorageOS.
//go:generate mockgen -build_flags=--mod=vendor -destination=mocks/mock_node_deleter.go -package=mocks . NodeDeleter
type NodeDeleter interface {
	DeleteNode(ctx context.Context, key client.ObjectKey) error
	ListNodes(ctx context.Context) ([]client.Object, error)
}

// Reconciler reconciles a Node object by deleting the StorageOS node object
// when the corresponding Kubernetes node is deleted.
type Reconciler struct {
	client.Client
	log        logr.Logger
	api        NodeDeleter
	gcDelay    time.Duration
	gcInterval time.Duration

	objectv1.Reconciler
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewReconciler returns a new Node delete reconciler.
//
// The gcInterval determines how often the periodic resync operation should be
// run.
func NewReconciler(api NodeDeleter, k8s client.Client, gcDelay time.Duration, gcInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:     k8s,
		log:        ctrl.Log,
		api:        api,
		gcDelay:    gcDelay,
		gcInterval: gcInterval,
	}
}

// SetupWithManager registers the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	c, err := NewController(r.api, r.log)
	if err != nil {
		return err
	}

	// Set the garbage collection interval.
	r.Reconciler.SetStartupGarbageCollectionDelay(r.gcDelay)
	r.Reconciler.SetGarbageCollectionPeriod(r.gcInterval)

	// Initialize the reconciler.
	err = r.Reconciler.Init(mgr, c, &corev1.Node{}, &corev1.NodeList{},
		syncv1.WithName("node-delete"),
		syncv1.WithLogger(r.log),
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
