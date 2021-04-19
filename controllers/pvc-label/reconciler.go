package pvclabel

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

// VolumeLabeller provides access to update volume labels.
//go:generate mockgen -build_flags=--mod=vendor -destination=mocks/mock_volume_labeller.go -package=mocks . VolumeLabeller
type VolumeLabeller interface {
	EnsureVolumeLabels(ctx context.Context, key client.ObjectKey, labels map[string]string) error
	VolumeObjects(ctx context.Context) (map[client.ObjectKey]storageos.Object, error)
}

// Reconciler reconciles a PVC by applying labels from the Kubernetes
// PVC to the StorageOS volume object.
type Reconciler struct {
	client.Client
	log            logr.Logger
	api            VolumeLabeller
	resyncDelay    time.Duration
	resyncInterval time.Duration

	msyncv1.Reconciler
}

// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch

// NewReconciler returns a new PVC label reconciler.
//
// The resyncInterval determines how often the periodic resync operation should
// be run.
func NewReconciler(api VolumeLabeller, k8s client.Client, resyncDelay time.Duration, resyncInterval time.Duration) *Reconciler {
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
	c, err := NewController(r.Client, r.api, mgr.GetScheme(), r.log)
	if err != nil {
		return err
	}

	// Set the resync interval.
	r.Reconciler.SetStartupSyncDelay(r.resyncDelay)
	r.Reconciler.SetResyncPeriod(r.resyncInterval)

	// Initialize the reconciler.
	err = r.Reconciler.Init(mgr, c, &corev1.PersistentVolumeClaim{}, &corev1.PersistentVolumeClaimList{},
		syncv1.WithName("volume-label-sync"),
		syncv1.WithLogger(r.log),
	)
	if err != nil {
		return fmt.Errorf("failed to create new reconciler: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		For(&corev1.PersistentVolumeClaim{}).
		WithEventFilter(Predicate{log: r.log}).
		Complete(r)
}
