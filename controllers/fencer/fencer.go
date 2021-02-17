package fencer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/storageos/api-manager/internal/pkg/annotation"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// gracePeriodSeconds is how long to wait for the Pod to be deleted
	// gracefully.  Since we expect the kubelet not to respond, 0 allows
	// immediate progression.
	gracePeriodSeconds = int64(0)
)

// fenceNode will kill Pods on the failed node that are using StorageOS volumes
// if the Pod has the fencing label set.  It will also remove the
// VolumeAttachments.
func (r *Reconciler) fenceNode(ctx context.Context, obj client.Object) error {
	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "fence node")
	span.SetAttributes(label.String("name", obj.GetName()))
	defer span.End()

	// Fetch pods running on failed node.
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingFields{"spec.nodeName": obj.GetName()}); err != nil {
		return err
	}

	// Fetch volume attachments for node.
	vaList := &storagev1.VolumeAttachmentList{}
	if err := r.List(ctx, vaList, client.MatchingFields{"spec.nodeName": obj.GetName()}); err != nil {
		return err
	}

	// Process each pod independently.
	for _, pod := range podList.Items {
		pod := pod
		log := r.log.WithValues("name", pod.GetName(), "namespace", pod.GetNamespace())
		log.V(4).Info("evaluating pod for fencing")

		// Ignore pods that don't have the fenced label set.
		fenced := false
		for k, v := range pod.GetLabels() {
			if k == storageos.ReservedLabelFencing {
				enabled, err := strconv.ParseBool(v)
				if err != nil {
					log.Error(err, "failed to parse enabled value for storageos.com/fenced label, expected true/false")
				}
				fenced = enabled
				break
			}
		}
		if !fenced {
			log.Info("skipping pod without storageos.com/fenced=true label set")
			continue
		}
		if err := r.fencePod(ctx, &pod, vaList); err != nil {
			log.Error(err, "failed to fence pod")
			continue
		}
		log.Info("fenced pod")
	}

	return nil
}

// fencePod performs all actions required to allow a Pod to be rescheduled
// immediately on another node, providing the prerequisites are met.
func (r *Reconciler) fencePod(ctx context.Context, pod *corev1.Pod, vaList *storagev1.VolumeAttachmentList) error {
	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "fence pod")
	span.SetAttributes(label.String("name", pod.GetName()), label.String("namespace", pod.GetNamespace()))
	defer span.End()
	log := r.log.WithValues("pod", pod.GetName(), "namespace", pod.GetNamespace())

	// All StorageOS volumes will be attached via PVCs since that is the only
	// method CSI supports (even for pre-provisioned PVs).
	pvcs, err := r.podPVCs(ctx, pod)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get pvcs for pod: %v", err)
	}
	volKeys := []types.NamespacedName{}
	for _, pvc := range pvcs {
		if pvc.Spec.VolumeName != "" {
			volKeys = append(volKeys, types.NamespacedName{Name: pvc.Spec.VolumeName, Namespace: pvc.Namespace})
		}
	}

	// Skip pod if no StorageOS PVCs or PVs.
	if len(volKeys) == 0 {
		span.SetStatus(codes.Ok, "no storageos pvcs")
		log.Info("skipping pod with no storageos pvcs")
		return nil
	}

	// Get list of StorageOS volumes that match the Pod's PVs.
	volumes, err := r.stosVolumes(ctx, volKeys)
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

	// Delete pod to allow it to be re-scheduled on another node.
	log.Info("pod has fenced label set and volume(s) still healthy after node failure, deleting pod")
	if err := r.Delete(ctx, pod, &client.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds}); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete pod: %v", err)
	}

	// Delete VolumeAttachments so the new Pod doesn't need to wait for expiry.
	var errors *multierror.Error
	for _, pvc := range pvcs {
		ctx, span := tr.Start(ctx, "delete volume attachment")
		span.SetAttributes(label.String("pvc", pvc.GetName()))
		defer span.End()
		log := log.WithValues("pvc", pvc.GetName())

		va := pvcVA(ctx, pvc, vaList)
		if va == nil {
			span.RecordError(err)
			log.Error(err, "skipping volume attachment removal")
			continue
		}
		span.SetAttributes(label.String("va", va.GetName()))
		log = log.WithValues("va", va.GetName())

		// This should never happen as we're only processing StorageOS PVCs, but check anyways.
		if va.Spec.Attacher != DriverName {
			span.RecordError(ErrUnexpectedVolumeAttacher)
			log.Error(ErrUnexpectedVolumeAttacher, "skipping volume attachment removal, expected storageos attacher")
			continue
		}

		if err := r.Delete(ctx, va, &client.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds}); err != nil {
			// Log failure and keep processing remaining attachments.
			span.RecordError(err)
			log.Error(err, "failed to delete volume attachment")
			errors = multierror.Append(err, errors)
		}
	}
	return errors.ErrorOrNil()
}

// podPVCs returns the StorageOS PVCs for a Pod.  Only PVCs that were
// dynamically provisioned using a StorageOS-based StorageClass or
// statically-provisioned using the StorageOS CSI Driver are returned.
func (r *Reconciler) podPVCs(ctx context.Context, pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcs []*corev1.PersistentVolumeClaim
	var errors *multierror.Error

	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "pod pvcs")
	span.SetAttributes(label.String("name", pod.GetName()), label.String("namespace", pod.GetNamespace()))
	defer span.End()
	log := r.log.WithValues("pod", pod.GetName(), "namespace", pod.GetNamespace())

	for _, vol := range pod.Spec.Volumes {
		ctx, span := tr.Start(ctx, "pod volume")
		span.SetAttributes(label.String("volume", vol.Name))
		defer span.End()
		if vol.PersistentVolumeClaim == nil || vol.PersistentVolumeClaim.ClaimName == "" {
			span.SetStatus(codes.Ok, "no pvc for this volume type")
			continue
		}
		span.SetAttributes(label.String("claim", vol.PersistentVolumeClaim.ClaimName))
		key := types.NamespacedName{
			Name:      vol.PersistentVolumeClaim.ClaimName,
			Namespace: pod.Namespace,
		}

		pvc := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, key, pvc); err != nil {
			span.RecordError(err)
			errors = multierror.Append(errors, err)
			continue
		}
		span.SetAttributes(label.String("spec", pvc.Spec.String()))

		// If the volume was dynamically provisioned, an annotation with the
		// driver name will be set.  This saves a PV lookup.
		provisioner, ok := pvc.Annotations[annotation.ProvisionerAnnotationKey]
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
		if err := r.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, pv); err != nil {
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
func (r *Reconciler) stosVolumes(ctx context.Context, keys []types.NamespacedName) ([]storageos.Object, error) {
	var volumes []storageos.Object

	tr := otel.Tracer("fencer")
	ctx, span := tr.Start(ctx, "storageos volumes")
	defer span.End()

	for _, key := range keys {
		volume, err := r.api.GetVolume(ctx, key)
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
