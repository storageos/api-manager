package nodelabel

import (
	"context"
	"fmt"
	"time"

	msyncv1 "github.com/darkowlzz/operator-toolkit/controller/metadata-sync/v1"
	syncv1 "github.com/darkowlzz/operator-toolkit/controller/sync/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// NodeLabeller provides access to update node labels.
//go:generate mockgen -destination=mocks/mock_node_labeller.go -package=mocks . NodeLabeller
type NodeLabeller interface {
	EnsureNodeLabels(ctx context.Context, name string, labels map[string]string) error
	NodeObjects(ctx context.Context) (map[string]storageos.Object, error)
}

// Reconciler reconciles a Node object by applying labels from the Kubernetes
// node to the StorageOS node object.
type Reconciler struct {
	client.Client
	log            logr.Logger
	api            NodeLabeller
	resyncDelay    time.Duration
	resyncInterval time.Duration

	msyncv1.Reconciler
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewReconciler returns a new Node label reconciler.
//
// The resyncInterval determines how often the periodic resync operation should
// be run.
func NewReconciler(api NodeLabeller, k8s client.Client, resyncDelay time.Duration, resyncInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:         k8s,
		log:            ctrl.Log,
		api:            api,
		resyncDelay:    resyncDelay,
		resyncInterval: resyncInterval,
	}
}

// SetupWithManager registers the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	c, err := NewController(r.api, r.log)
	if err != nil {
		return err
	}

	// Set the resync interval.
	r.Reconciler.SetStartupSyncDelay(r.resyncDelay)
	r.Reconciler.SetResyncPeriod(r.resyncInterval)

	// Initialize the reconciler.
	err = r.Reconciler.Init(mgr, c, &corev1.Node{}, &corev1.NodeList{},
		syncv1.WithName("node-label-sync"),
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
