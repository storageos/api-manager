package fencer

import (
	"context"
	"time"

	"github.com/darkowlzz/operator-toolkit/controller/external/builder"
	"github.com/darkowlzz/operator-toolkit/controller/external/handler"
	actionv1 "github.com/darkowlzz/operator-toolkit/controller/stateless-action/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/storageos/api-manager/internal/pkg/cache"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

const (
	// minPollInterval will override more frequent intervals to protect the
	// backend api.
	minPollInterval = 5 * time.Second
)

var (
	// ErrNodeNotCached is returned if the node was expected in the cache but
	// not found.
	ErrNodeNotCached = errors.New("node not found in cache")
)

// NodeFencer provides access to nodes and the volumes running on them.
//go:generate mockgen -destination=mocks/mock_node_fencer.go -package=mocks . NodeFencer
type NodeFencer interface {
	ListNodes(ctx context.Context) ([]client.Object, error)
	GetVolume(ctx context.Context, key client.ObjectKey) (storageos.Object, error)
}

// Reconciler reconciles StorageOS Node object health with running Pods,
// deleting them if we know that they are unable to use their storage.
type Reconciler struct {
	client.Client
	scheme          *runtime.Scheme
	log             logr.Logger
	api             NodeFencer
	apiReset        chan<- struct{}
	pollInterval    time.Duration
	expiryInterval  time.Duration
	cleanupInterval time.Duration
	cache           *cache.Object

	actionv1.Reconciler
}

// +kubebuilder:rbac:groups="",resources=node,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=volumeattachments,verbs=get;list;watch;delete

// NewReconciler returns a new Node label reconciler.
//
// The resyncInterval determines how often the periodic resync operation should
// be run.
func NewReconciler(api NodeFencer, apiReset chan<- struct{}, k8s client.Client, pollInterval time.Duration, expiryInterval time.Duration) *Reconciler {
	// Don't allow an overly-agressive interval to break the backend.
	if pollInterval < minPollInterval {
		ctrl.Log.Info("resetting poll interval to minimum", "interval", minPollInterval)
		pollInterval = minPollInterval
	}

	return &Reconciler{
		Client:          k8s,
		log:             ctrl.Log,
		api:             api,
		apiReset:        apiReset,
		pollInterval:    pollInterval,
		expiryInterval:  expiryInterval,
		cleanupInterval: 5 * expiryInterval,
	}
}

func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, workers int, retryInterval time.Duration, timeout time.Duration) error {
	// Create an generic event source. This is used by the Channel type source
	// to collect the events and process with source event handler.
	src := make(chan event.GenericEvent)

	// Set scheme for conversions.
	r.scheme = mgr.GetScheme()

	// Initialize the cache.
	r.cache = cache.New(r.expiryInterval, r.cleanupInterval)

	// Create an event handler that uses the cache to make reconciliation
	// decisions.
	eventHandler := handler.NewEnqueueRequestFromCache(r.cache)

	// Periodically populate the cache by sending StorageOS API nodes to the
	// event source.
	go r.pollNodes(ctx, src)

	// Create an index on the Pod's node name.
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}

	// Create an index on the VolumeAttachment's node name.
	if err := mgr.GetFieldIndexer().IndexField(ctx, &storagev1.VolumeAttachment{}, "spec.nodeName", func(rawObj client.Object) []string {
		va := rawObj.(*storagev1.VolumeAttachment)
		return []string{va.Spec.NodeName}
	}); err != nil {
		return err
	}

	// Create a new fencing controller.
	c, err := NewController(r.Client, r.cache, mgr.GetScheme(), r.api, r.log)
	if err != nil {
		return err
	}

	// Initialize the reconciler with the fencing controller.
	r.Reconciler.Init(mgr, c,
		actionv1.WithName("pod-fencer"),
		actionv1.WithScheme(mgr.GetScheme()),
		actionv1.WithActionTimeout(timeout),
		actionv1.WithActionRetryPeriod(retryInterval),
		actionv1.WithLogger(r.log),
	)

	return builder.ControllerManagedBy(mgr).
		Named("fencer").
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		WithSource(src).
		WithEventHandler(eventHandler).
		Complete(r)
}

// pollNodes periodically retrieves the StorageOS nodes and sends them over the
// source event channel.  It is blocking and is intended to run in a goroutine,
// stopping when the context is cancelled.
func (r *Reconciler) pollNodes(ctx context.Context, src chan event.GenericEvent) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tr := otel.Tracer("fencer")
			ctx, span := tr.Start(ctx, "fencing poller")
			r.log.V(5).Info("polling for external node health")

			nodes, err := r.api.ListNodes(ctx)
			if err != nil {
				span.RecordError(errors.Wrap(err, "failed to list nodes"))
				r.log.Error(err, "failed to list nodes")

				// We should always be able to list nodes.  Reset the api client
				// if we got an error.
				r.apiReset <- struct{}{}

				span.End()
				continue
			}
			span.SetAttributes(label.Int("nodes", len(nodes)))
			for _, node := range nodes {
				src <- event.GenericEvent{
					Object: node,
				}
			}
			span.SetStatus(codes.Ok, "refreshed node cache")
			span.End()
		case <-ctx.Done():
			return
		}
	}
}
