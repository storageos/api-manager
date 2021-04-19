package fencer

import (
	"context"
	"fmt"
	"strconv"

	actionv1 "github.com/darkowlzz/operator-toolkit/controller/stateless-action/v1"
	"github.com/darkowlzz/operator-toolkit/controller/stateless-action/v1/action"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storageosv1 "github.com/storageos/api-manager/api/v1"
	"github.com/storageos/api-manager/internal/pkg/cache"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

const (
	// DriverName is the name of the StorageOS CSI driver.
	DriverName = "csi.storageos.com"
)

var (
	// ErrVolumeAttachmentNotFound is returned when a volume attachment was
	// expected but not found.
	ErrVolumeAttachmentNotFound = errors.New("volume attachment not found")

	// ErrUnexpectedVolumeAttacher is returned when a specific attacher
	// was expected but different or not specified.
	ErrUnexpectedVolumeAttacher = errors.New("unexpected volume attacher")

	// ErrNodeTypeAssertion is returned when a type assertion to convert a
	// given object into StorageOS Node fails.
	ErrNodeTypeAssertion = errors.New("failed to convert into StorageOS Node by type assertion")

	// gracePeriodSeconds is how long to wait for the Pod to be deleted
	// gracefully.  Since we expect the kubelet not to respond, 0 allows
	// immediate progression.
	gracePeriodSeconds = int64(0)
)

// Controller implements the Stateless-Action controller interface, fencing k8s
// node pods when they are detected to be unhealthy in StorageOS.
type Controller struct {
	client.Client
	api    NodeFencer
	log    logr.Logger
	cache  *cache.Object
	scheme *runtime.Scheme
}

var _ actionv1.Controller = &Controller{}

// NewController returns a Controller that implements pod fencing based on
// StorageOS node health status.
func NewController(k8s client.Client, cache *cache.Object, scheme *runtime.Scheme, api NodeFencer, log logr.Logger) (*Controller, error) {
	return &Controller{Client: k8s, api: api, log: log, cache: cache, scheme: scheme}, nil
}

func (c Controller) GetObject(ctx context.Context, key client.ObjectKey) (interface{}, error) {
	tr := otel.Tracer("fencer")
	_, span := tr.Start(ctx, "get object")
	span.SetAttributes(label.String("key", key.String()))
	defer span.End()

	// Query the object details from the cache.
	val, found := c.cache.Get(key.String())
	if !found {
		span.RecordError(ErrNodeNotCached)
		return nil, ErrNodeNotCached
	}
	node := &storageosv1.Node{}
	if err := c.scheme.Convert(val, node, nil); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to convert cached object to Node: %v", err)
	}
	return node, nil
}

func (c Controller) RequireAction(ctx context.Context, o interface{}) (bool, error) {
	tr := otel.Tracer("fencer")
	_, span := tr.Start(ctx, "require action")
	defer span.End()

	node, ok := o.(*storageosv1.Node)
	if !ok {
		span.RecordError(ErrNodeTypeAssertion)
		return false, ErrNodeTypeAssertion
	}
	span.SetAttributes(label.String("node", node.GetName()))

	// Check if an associated k8s node exists. Action not required if k8s node
	// doesn't exist.
	k8sNode := &corev1.Node{}
	if err := c.Get(ctx, client.ObjectKey{Name: node.GetName()}, k8sNode); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Unhealthy node require action.
	if node.Status.Health == storageosv1.NodeHealthOffline {
		span.AddEvent("Node offline")
		return true, nil
	}

	c.log.V(5).Info("ignore healthy node")
	span.SetStatus(codes.Ok, "node healthy")
	return false, nil
}

func (c Controller) BuildActionManager(o interface{}) (action.Manager, error) {
	node, ok := o.(*storageosv1.Node)
	if !ok {
		return nil, fmt.Errorf("failed to convert into StorageOS Node")
	}

	return &fenceActionManager{
		Client: c.Client,
		api:    c.api,
		log:    c.log,
		node:   node,
		cache:  c.cache,
		scheme: c.scheme,
	}, nil
}

// fenceActionManager implements the action manager interface for fencing
// stateless-action controller. The controller creates fence action managers
// with an action definition. The action manager performs the action.
type fenceActionManager struct {
	client.Client
	api    NodeFencer
	log    logr.Logger
	cache  *cache.Object
	scheme *runtime.Scheme

	// node is the target node that's being fenced.
	node *storageosv1.Node
}

func (am fenceActionManager) GetName(o interface{}) (string, error) {
	return am.node.Name, nil
}

func (am fenceActionManager) GetObjects(context.Context) ([]interface{}, error) {
	// Return the node itself because the fenceNode() handles finding all the
	// pods on the node and fences them.
	// NOTE: Alternatively, all the pods can be queried and filtered here,
	// returning only the list of pods that are to be fenced. Run() can just
	// ensure that the pod is deleted from the target node.
	return []interface{}{am.node}, nil
}

func (am fenceActionManager) Run(ctx context.Context, o interface{}) error {
	// NOTE: o is not converted into a node because this action manager is very
	// specific to an object. For cases where the action is run on different
	// objects derived from a target object, like fencing individual pods that
	// are associated with the target node, o will be different from the target
	// object and should be converted to the appropriate type and used.
	return am.fenceNode(ctx, am.node)
}

func (am fenceActionManager) Defer(context.Context, interface{}) error {
	return nil
}

// Check queries the world to find out if the fencing was successful or it
// should be rerun. It returns true if a rerun is needed.
func (am fenceActionManager) Check(ctx context.Context, o interface{}) (bool, error) {
	// Fetch the latest storageos node health info and check if it's still
	// unhealthy.
	key := client.ObjectKeyFromObject(am.node)
	val, found := am.cache.Get(key.String())
	if !found {
		return true, ErrNodeNotCached
	}

	if err := am.scheme.Convert(val, am.node, nil); err != nil {
		return true, errors.Wrap(err, "failed to convert cached object to Node")
	}

	// If the node is no longer offline, action is no longer needed.
	if am.node.Status.Health != storageosv1.NodeHealthOffline {
		return false, nil
	}

	// Get all the target pods. If there are target pods, return true, some
	// pods need to be fenced again.
	podList, err := am.getTargetPods(ctx)
	if err != nil {
		return true, errors.Wrap(err, "failed to get target pods to fence")
	}
	if len(podList.Items) > 0 {
		return true, nil
	}

	return false, nil
}

// getTargetPods returns a PodList of the pods that are on the target node and
// have volumes provisioned by storageos.
func (am fenceActionManager) getTargetPods(ctx context.Context) (*corev1.PodList, error) {
	needFencingPodList := &corev1.PodList{}

	// Fetch pods running on failed node.
	podList := &corev1.PodList{}
	if err := am.List(ctx, podList, client.MatchingFields{"spec.nodeName": am.node.GetName()}); err != nil {
		return podList, err
	}

	for _, apod := range podList.Items {
		pod := apod
		log := am.log.WithValues("name", pod.GetName(), "namespace", pod.GetNamespace())
		log.V(4).Info("evaluating pod for fencing")

		// Ignore pods that don't have the fenced label set.
		fenced := false
		if v, ok := pod.Labels[storageos.ReservedLabelFencing]; ok {
			enabled, err := strconv.ParseBool(v)
			if err != nil {
				log.Error(err, "failed to parse enabled value for storageos.com/fenced label, expected true/false")
			}
			fenced = enabled
		}
		if !fenced {
			log.Info("skipping pod without storageos.com/fenced=true label set")
			continue
		}

		// Check if the pod's PVCs are provisioned by storageos.

		// All StorageOS volumes will be attached via PVCs since that is the only
		// method CSI supports (even for pre-provisioned PVs).
		pvcs, err := am.podPVCs(ctx, &pod)
		if err != nil {
			// span.RecordError(err)
			return nil, fmt.Errorf("failed to get pvcs for pod: %v", err)
		}
		volKeys := []client.ObjectKey{}
		for _, pvc := range pvcs {
			if pvc.Spec.VolumeName != "" {
				volKeys = append(volKeys, client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.Namespace})
			}
		}

		// Skip pod if no StorageOS PVCs or PVs.
		if len(volKeys) == 0 {
			continue
		}

		needFencingPodList.Items = append(needFencingPodList.Items, pod)
	}

	return needFencingPodList, nil
}

// fenceNode will kill Pods on the failed node that are using StorageOS volumes
// if the Pod has the fencing label set.  It will also remove the
// VolumeAttachments.
func (am *fenceActionManager) fenceNode(ctx context.Context, obj client.Object) error {
	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "fence node")
	span.SetAttributes(label.String("name", obj.GetName()))
	defer span.End()

	// Fetch volume attachments for node.
	vaList := &storagev1.VolumeAttachmentList{}
	if err := am.List(ctx, vaList, client.MatchingFields{"spec.nodeName": obj.GetName()}); err != nil {
		return err
	}

	podList, err := am.getTargetPods(ctx)
	if err != nil {
		return err
	}

	// Process each pod independently.
	for _, pod := range podList.Items {
		pod := pod
		if err := am.fencePod(ctx, &pod, vaList); err != nil {
			am.log.Error(err, "failed to fence pod")
			continue
		}
		am.log.Info("fenced pod")
	}

	return nil
}

// fencePod performs all actions required to allow a Pod to be rescheduled
// immediately on another node, providing the prerequisites are met.
func (am *fenceActionManager) fencePod(ctx context.Context, pod *corev1.Pod, vaList *storagev1.VolumeAttachmentList) error {
	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "fence pod")
	span.SetAttributes(label.String("name", pod.GetName()), label.String("namespace", pod.GetNamespace()))
	defer span.End()
	log := am.log.WithValues("pod", pod.GetName(), "namespace", pod.GetNamespace())

	// All StorageOS volumes will be attached via PVCs since that is the only
	// method CSI supports (even for pre-provisioned PVs).
	pvcs, err := am.podPVCs(ctx, pod)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get pvcs for pod: %v", err)
	}
	volKeys := []client.ObjectKey{}
	for _, pvc := range pvcs {
		if pvc.Spec.VolumeName != "" {
			volKeys = append(volKeys, client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.Namespace})
		}
	}

	// Get list of StorageOS volumes that match the Pod's PVs.
	volumes, err := am.stosVolumes(ctx, volKeys)
	if err != nil {
		return fmt.Errorf("failed to get storageos volumes for pod: %v", err)
	}

	// All volume masters need to be healthy for pod failover to succeed, so ignore
	// until they are.
	canFailover := true
	for _, volume := range volumes {
		if !volume.IsHealthy() {
			log.Info("fencer skipping pod with unhealthy volume", "volume", volume.GetName())
			canFailover = false
			break
		}
	}
	if !canFailover {
		span.SetStatus(codes.Ok, "unhealthy volume(s)")
		log.Info("pod has been labeled as fenced but not all volumes are healthy after node failure, leaving pod running")
		return nil
	}

	log.Info("pod has fenced label set and volume(s) still healthy after node failure, proceeding with fencing")

	// Delete pod to allow it to be rescheduled on another node.
	if err := am.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete pod: %v", err)
	}
	span.AddEvent("pod deleted")
	log.Info("pod deleted")

	// Delete the VolumeAttachments.  This allows the rescheduled Pod to mount
	// its volumes almost immediately, without waiting for them to expire.
	//
	// It's likely that the rescheduled Pod will try to attach its volumes
	// before the old VolumeAttachments have been removed.  This will be shown
	// as a "Multi-attach" error.
	//
	// This can't be avoided by deleting the VolumeAttachments before the Pod as
	// they'll get recreated almost instantly.
	//
	// Any errors deleting the VA will not stop processing or force a requeue.
	// Instead, failover will take longer waiting for the VA to expire.
	for _, pvc := range pvcs {
		ctx, span := tr.Start(ctx, "delete volume attachment")
		span.SetAttributes(label.String("pvc", pvc.GetName()))
		defer span.End()
		log := log.WithValues("pvc", pvc.GetName())

		va := pvcVA(ctx, pvc, vaList)
		if va == nil {
			span.AddEvent("no volume attachment found for pvc")
			log.Info("no volume attachment found for pvc")
			continue
		}
		span.SetAttributes(label.String("va", va.GetName()), label.String("node", va.Spec.NodeName))
		log = log.WithValues("va", va.GetName(), "node", va.Spec.NodeName)

		// This should never happen as we're only processing StorageOS PVCs, but
		// check anyways.
		if va.Spec.Attacher != DriverName {
			span.RecordError(ErrUnexpectedVolumeAttacher)
			log.Error(ErrUnexpectedVolumeAttacher, "expected storageos attacher")
			continue
		}

		if err := am.Delete(ctx, va, &client.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds}); err != nil {
			span.RecordError(err)
			log.Error(err, "failed to delete volume attachment")
			continue
		}
		span.AddEvent("volume attachment deleted")
		log.Info("volume attachment deleted")
	}

	return nil
}

// podPVCs returns the StorageOS PVCs for a Pod.  Only PVCs that were
// dynamically provisioned using a StorageOS-based StorageClass or
// statically-provisioned using the StorageOS CSI Driver are returned.
func (am *fenceActionManager) podPVCs(ctx context.Context, pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcs []*corev1.PersistentVolumeClaim
	var errors *multierror.Error

	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "pod pvcs")
	span.SetAttributes(label.String("name", pod.GetName()), label.String("namespace", pod.GetNamespace()))
	defer span.End()
	log := am.log.WithValues("pod", pod.GetName(), "namespace", pod.GetNamespace())

	for _, vol := range pod.Spec.Volumes {
		ctx, span := tr.Start(ctx, "pod volume")
		span.SetAttributes(label.String("volume", vol.Name))
		defer span.End()
		if vol.PersistentVolumeClaim == nil || vol.PersistentVolumeClaim.ClaimName == "" {
			span.SetStatus(codes.Ok, "no pvc for this volume type")
			continue
		}
		span.SetAttributes(label.String("claim", vol.PersistentVolumeClaim.ClaimName))
		key := client.ObjectKey{
			Name:      vol.PersistentVolumeClaim.ClaimName,
			Namespace: pod.Namespace,
		}

		pvc := &corev1.PersistentVolumeClaim{}
		if err := am.Get(ctx, key, pvc); err != nil {
			span.RecordError(err)
			errors = multierror.Append(errors, err)
			continue
		}
		span.SetAttributes(label.String("spec", pvc.Spec.String()))

		// If the volume was dynamically provisioned, an annotation with the
		// driver name will be set.  This saves a PV lookup.
		provisioner, ok := pvc.Annotations[provisioner.PVCProvisionerAnnotationKey]
		if ok {
			span.SetAttributes(label.String("provisioner", provisioner))
			if provisioner == DriverName {
				pvcs = append(pvcs, pvc)
			}
			continue
		}

		// If no annotation was set, the volume may have been statically
		// provisioned. Lookup the PV to read the provisioner from the
		// VolumeSource.
		pv := &corev1.PersistentVolume{}
		if err := am.Get(ctx, client.ObjectKey{Name: pvc.Spec.VolumeName}, pv); err != nil {
			// If the PV was not found then we can not be sure it wasn't a
			// StorageOS volume.  However, since it would not be mountable then
			// it doesn't matter, and we can ignore the volume.
			span.RecordError(err)
			log.WithValues("volume_name", pvc.Spec.VolumeName).Info("Ignoring volume that was not dynamically provisioned by csi and no pv found")
			continue
		}
		if pv.Spec.CSI != nil {
			span.SetAttributes(label.String("pv provisioner", provisioner))
			if pv.Spec.CSI.Driver == DriverName {
				pvcs = append(pvcs, pvc)
			}
		}
	}
	return pvcs, errors.ErrorOrNil()
}

// stosVolumes returns a list of StorageOS volume objects that correspond to the
// given keys.  We need all volumes, so return on any error.
func (am *fenceActionManager) stosVolumes(ctx context.Context, keys []client.ObjectKey) ([]storageos.Object, error) {
	var volumes []storageos.Object

	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "storageos volumes")
	defer span.End()

	for _, key := range keys {
		volume, err := am.api.GetVolume(ctx, key)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to get storageos volume for pvc: %v", err)
		}
		volumes = append(volumes, volume)
	}
	return volumes, nil
}

// pvcVA returns the VolumeAttachment for a PVC, given a list of VolumeAttachements.
func pvcVA(ctx context.Context, pvc *corev1.PersistentVolumeClaim, vaList *storagev1.VolumeAttachmentList) *storagev1.VolumeAttachment {
	tr := otel.Tracer("fencer")
	_, span := tr.Start(ctx, "volume attachment")
	defer span.End()

	for _, va := range vaList.Items {
		if *va.Spec.Source.PersistentVolumeName == pvc.Spec.VolumeName {
			span.SetAttributes(label.String("name", va.GetName()))
			span.SetAttributes(label.String("attacher", va.Spec.Attacher))
			span.SetAttributes(label.String("node", va.Spec.NodeName))
			span.SetAttributes(label.String("pv", *va.Spec.Source.PersistentVolumeName))
			span.SetAttributes(label.Bool("attached", va.Status.Attached))
			return &va
		}
	}
	return nil
}
