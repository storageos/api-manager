package fencer

import (
	"context"
	"fmt"
	"time"

	"github.com/darkowlzz/operator-toolkit/controller/external/builder"
	"github.com/darkowlzz/operator-toolkit/controller/external/handler"
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

	storageosv1 "github.com/storageos/api-manager/api/v1"
	"github.com/storageos/api-manager/internal/pkg/cache"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

var (
	// ErrNodeNotCached is returned if the node was expected in the cache but
	// not found.
	ErrNodeNotCached = errors.New("node not found in cache")
)

// NodeFencer provides access to nodes and the volumes running on them.
//go:generate mockgen -destination=mocks/mock_node_fencer.go -package=mocks . NodeFencer
type NodeFencer interface {
	ListNodes(ctx context.Context) ([]storageosv1.Node, error)
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

func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, workers int) error {
	// Create an generic event source. This is used by the Channel type source
	// to collect the events and process with source event handler.
	src := make(chan event.GenericEvent)

	// Set scheme for conversions.
	r.scheme = mgr.GetScheme()

	// Initialize the cache.
	r.cache = cache.New(r.expiryInterval, r.cleanupInterval, r.log)

	// Create an event handler that uses the cache to make reconciliation
	// decisions.
	eventHandler := handler.NewEnqueueRequestFromCache(r.cache)

	// Periodically populate the cache from StorageOS API nodes.
	go func() {
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

					r.apiReset <- struct{}{}
					span.End()
					continue
				}
				span.SetAttributes(label.Int("nodes", len(nodes)))
				for _, node := range nodes {
					var node = node
					src <- event.GenericEvent{
						Object: &node,
					}
				}
				span.SetStatus(codes.Ok, "refreshed node cache")
				span.End()
			case <-ctx.Done():
				return
			}
		}
	}()

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

	return builder.ControllerManagedBy(mgr).
		Named("fencer").
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		WithSource(src).
		WithEventHandler(eventHandler).
		Complete(r)
}

// Reconcile is part of the main node health reconcile loop which aims to move
// the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "node fencer reconcile")
	span.SetAttributes(label.String("name", req.NamespacedName.Name))
	defer span.End()
	log := r.log.WithValues("name", req.NamespacedName)

	// Query the object details from the cache and operate on it.
	val, found := r.cache.Get(req.NamespacedName.String())
	if !found {
		log.Error(ErrNodeNotCached, "skipping node, will retry")
		span.RecordError(ErrNodeNotCached)
		return ctrl.Result{}, nil
	}
	node := &storageosv1.Node{}
	if err := r.scheme.Convert(val, node, nil); err != nil {
		span.RecordError(err)
		return ctrl.Result{}, fmt.Errorf("failed to convert cached object to Node: %v", err)
	}

	// Ignore healthy nodes
	if node.Status.Health != storageosv1.NodeHealthOffline {
		log.V(5).Info("ignore healthy node")
		span.SetStatus(codes.Ok, "node healthy")
		return ctrl.Result{}, nil
	}

	log.Info("fencing pods on node")
	if err := r.fenceNode(ctx, node); err != nil {
		span.RecordError(err)
		return ctrl.Result{}, fmt.Errorf("failed to fence node: %v", err)
	}
	span.SetStatus(codes.Ok, "fenced pods on node")
	return ctrl.Result{}, nil
}
