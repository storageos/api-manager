package sharedvolume

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	cache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/storageos"
)

const (
	// defaultCacheExpiryInterval determines how long a cached item will persist
	// for without being refreshed.  Once it expires, the service will be
	// fetched and compared to ensure it has not drifted.  This is configurable
	// with the `-cache-expiry-interval` flag.
	defaultCacheExpiryInterval = 1 * time.Minute

	// defaultCacheCleanupInterval determines how often the cache is checked for
	// expired items and removes them from the cache.
	defaultCacheCleanupInterval = 5 * time.Minute
)

var (
	// ErrCastCache is returned if a cache entry could not be converted to the
	// expected object type.
	ErrCastCache = errors.New("failed to cast object from cache")
)

// Reconciler reconciles a SharedVolume object by creating the Kubernetes
// services that it requires to operate.
type Reconciler struct {
	client.Client
	log                   logr.Logger
	api                   storageos.VolumeSharer
	apiReset              chan<- struct{}
	apiPollInterval       time.Duration
	cacheExpiryInterval   time.Duration
	k8sCreatePollInterval time.Duration
	k8sCreateWaitDuration time.Duration
	volumes               *cache.Cache
	recorder              record.EventRecorder
}

// NewReconciler returns a new SharedVolumeAPIReconciler.
func NewReconciler(
	api storageos.VolumeSharer,
	apiReset chan<- struct{},
	k8s client.Client,
	apiPollInterval time.Duration,
	cacheExpiryInterval time.Duration,
	k8sCreatePollInterval time.Duration,
	k8sCreateWaitDuration time.Duration,
	recorder record.EventRecorder) *Reconciler {
	// Register prometheus metrics.
	RegisterMetrics()

	return &Reconciler{
		Client:                k8s,
		log:                   ctrl.Log.WithName("controllers").WithName("SharedVolume"),
		api:                   api,
		apiReset:              apiReset,
		apiPollInterval:       apiPollInterval,
		cacheExpiryInterval:   cacheExpiryInterval,
		k8sCreatePollInterval: k8sCreatePollInterval,
		k8sCreateWaitDuration: k8sCreateWaitDuration,
		volumes:               cache.New(defaultCacheExpiryInterval, defaultCacheCleanupInterval),
		recorder:              recorder,
	}
}

// SetupWithManager registers with the controller manager.
//
// Since this is an external controller, we don't need to register the
// controller, just add it as a Runnable so that the manager can control startup
// and shutdown.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return mgr.Add(r)
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=endpoints/status,verbs=get;update;patch

// Start runs the main reconcile loop until the context is cancelled or there is
// a fatal error.  It implements the controller-runtime Runnable interface so
// that it can be controlled by controller manager.
//
// It polls the StorageOS api every apiPollInterval and ensures that the K8s
// objects that are required for shared volumes are present.  It does not handle
// deleting objects that are no longer required, this is done via
// OwnerReferences.
func (r *Reconciler) Start(ctx context.Context) error {
	for {
		tr := otel.Tracer("shared-volume")
		ctx, span := tr.Start(ctx, "shared volume reconciler")

		start := time.Now()

		// Query shared volumes info.
		volumes, err := r.api.ListSharedVolumes(ctx)
		if err != nil {
			span.RecordError(errors.Wrap(err, "failed to list shared volumes"))
			r.log.Error(err, "failed to list shared volumes")
			r.apiReset <- struct{}{}
		}
		span.SetAttributes(label.Int("volumes", len(volumes)))

		for _, vol := range volumes {
			log := r.log.WithValues("svc", vol.ServiceName, "pvc", vol.PVCName, "namespace", vol.Namespace)

			// New tracing span for each volume.
			ctx, span := tr.Start(ctx, "reconcile shared volume")
			span.SetAttributes(label.String("pvc", vol.PVCName))
			span.SetAttributes(label.String("namespace", vol.Namespace))

			observeErr := func(err error, msg string) {
				span.RecordError(errors.Wrap(err, msg))
				log.Error(err, msg)
			}

			// Fetch volume from cache. If the cached entry is the same then
			// skip the k8s api requests to verify until the cache entry
			// expires.
			obj, found := r.volumes.Get(vol.ID)
			if found {
				cachedVol, ok := obj.(*storageos.SharedVolume)
				if !ok {
					log.Error(ErrCastCache, "failed to cast cache object to SharedVolume type", "object", obj)
					span.RecordError(err)
					span.End()
					continue
				}
				if cachedVol.IsEqual(vol) {
					span.AddEvent("no update needed")
					span.End()
					continue
				}
			}

			// Volume not cached or cached but expired or update needed.

			// Load the pvc for the SharedVolume - it will be set as the
			// service's owner reference.  If it doesn't exist then the service
			// is no longer required and we can ignore the request.
			pvc := &corev1.PersistentVolumeClaim{}
			if err := r.Client.Get(ctx, types.NamespacedName{Name: vol.PVCName, Namespace: vol.Namespace}, pvc); err != nil {
				if !apierrors.IsNotFound(err) {
					observeErr(err, "failed to fetch pvc for shared volume")
					span.End()
					continue
				}
			}
			ownerRef := metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
				Name:       pvc.Name,
				UID:        pvc.UID,
			}
			externalEndpoint, err := r.ensureService(ctx, vol, ownerRef, r.k8sCreatePollInterval, r.k8sCreateWaitDuration)
			if err != nil {
				observeErr(err, "shared volume create/update failed")
				span.End()
				continue
			}

			if externalEndpoint != vol.ExternalEndpoint {
				if err := r.api.SetExternalEndpoint(ctx, vol.ID, vol.Namespace, externalEndpoint); err != nil {
					observeErr(err, "shared volume external endpoint update failed")
					span.End()
					continue
				}
				log.Info("shared volume ready for use", "external", externalEndpoint)
				span.AddEvent("shared volume ready for use")
				vol.ExternalEndpoint = externalEndpoint
			}

			// Create/update/verify succeeded, update cache including resetting
			// expiry.
			r.volumes.Set(vol.ID, vol, r.cacheExpiryInterval)

			span.SetStatus(codes.Ok, "shared volume reconciled")
			span.End()
		}

		// Record reconcile duration.
		ReconcileDuration.Observe(time.Since(start))

		span.End()

		// Wait before polling again or exit if the context has been cancelled.
		select {
		case <-time.After(r.apiPollInterval):
		case <-ctx.Done():
			// Graceful shutdown, don't return error.
			return nil
		}
	}
}

// ensureService makes sure that the required k8s objects are up-to-date for the
// given SharedVolume.  Returns the public endpoint for the service.
func (r *Reconciler) ensureService(ctx context.Context, sv *storageos.SharedVolume, ownerRef metav1.OwnerReference, k8sCreatePollInterval time.Duration, k8sCreateWaitDuration time.Duration) (string, error) {
	tr := otel.Tracer("shared-volume")
	ctx, span := tr.Start(ctx, "ensure shared volume service")
	span.SetAttributes(label.String("pvc", sv.PVCName))
	span.SetAttributes(label.String("namespace", sv.Namespace))
	defer span.End()

	observeErr := func(err error, msg string) error {
		e := errors.Wrap(err, msg)
		span.RecordError(e)
		return e
	}

	nn := types.NamespacedName{
		Name:      sv.ServiceName,
		Namespace: sv.Namespace,
	}
	log := r.log.WithValues("svc", sv.ServiceName, "pvc", sv.PVCName, "namespace", sv.Namespace)

	svc := &corev1.Service{}
	err := r.Client.Get(ctx, nn, svc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Client.Create(ctx, sv.Service(ownerRef)); err != nil {
				return "", observeErr(err, "failed to create service resource")
			}
			if err := r.waitForClusterIP(ctx, nn, svc, k8sCreatePollInterval, k8sCreateWaitDuration); err != nil {
				return "", observeErr(err, "failed to get service resource after create")
			}
			span.AddEvent("shared volume service created")
			log.Info("shared volume service created", "external", frontend(svc))
			r.recorder.Event(svc, "Normal", "Created", fmt.Sprintf("Created service for shared volume %s/%s", sv.Namespace, sv.ServiceName))
		} else {
			return "", observeErr(err, "failed to get service, aborting reconcile")
		}
	}
	if !sv.ServiceIsEqual(svc) {
		if err := r.Client.Update(ctx, sv.ServiceUpdate(svc), &client.UpdateOptions{}); err != nil {
			return "", observeErr(err, "failed to update service resource")
		}
		span.AddEvent("shared volume service updated")
		log.Info("shared volume service updated", "external", frontend(svc))
	}

	ep := &corev1.Endpoints{}
	err = r.Client.Get(ctx, nn, ep)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Client.Create(ctx, sv.Endpoints()); err != nil {
				return "", observeErr(err, "failed to create endpoints resource")
			}
			if err := r.waitForAvailable(ctx, nn, ep, k8sCreatePollInterval, k8sCreateWaitDuration); err != nil {
				return "", observeErr(err, "failed to get endpoints resource after create")
			}
			span.AddEvent("shared volume endpoint created")
			log.Info("shared volume endpoint created", "internal", sv.InternalEndpoint)
		} else {
			return "", observeErr(err, "failed to get endpoint, aborting reconcile")
		}
	}
	if !sv.EndpointsIsEqual(ep) {
		if err := r.Client.Update(ctx, sv.EndpointsUpdate(ep), &client.UpdateOptions{}); err != nil {
			return "", observeErr(err, "failed to update endpoints resource")
		}
		span.AddEvent("shared volume endpoint updated")
		log.Info("shared volume endpoint updated", "internal", sv.InternalEndpoint)
		r.recorder.Event(svc, "Warning", "Updated", fmt.Sprintf("Shared volume service target changed %s/%s", sv.Namespace, sv.ServiceName))
	}

	endpoint := frontend(svc)
	span.SetAttributes(label.String("endpoint", endpoint))
	span.SetStatus(codes.Ok, "shared volume service configured")

	return endpoint, nil
}

// waitForClusterIP polls at the set interval until the timeout for the service
// to be found in the api with a ClusterIP set.
func (r *Reconciler) waitForClusterIP(ctx context.Context, nn types.NamespacedName, svc *corev1.Service, interval time.Duration, timeout time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		err := r.Client.Get(ctx, nn, svc)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		if svc.Spec.ClusterIP == "" {
			return false, nil
		}
		return true, nil
	})
}

// waitForAvailable fetches a Kubernetes object, polling at the set interval
// until the timeout for the object to be found.  This is useful for when the
// object was just created and you want to read it back immediately.
func (r *Reconciler) waitForAvailable(ctx context.Context, nn types.NamespacedName, obj client.Object, interval time.Duration, timeout time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		err := r.Client.Get(ctx, nn, obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, nil
	})
}

// frontend returns a service's public endpoint.
func frontend(svc *corev1.Service) string {
	if svc == nil || len(svc.Spec.Ports) != 1 {
		return ""
	}
	return fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, svc.Spec.Ports[0].Port)
}
