// Package provisioner provides information about the StorageOS provisioner.
package provisioner

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrUnsupportedKind is returned if the object kind being checked is not provisionable.
	ErrUnsupportedKind = errors.New("only nodes and pvcs have a provisioner set")
)

const (
	// DriverName is the name of the StorageOS CSI driver.
	DriverName = "csi.storageos.com"

	// NodeDriverAnnotationKey is the Node annotation that stores the registered CSI
	// drivers.
	NodeDriverAnnotationKey = "csi.volume.kubernetes.io/nodeid"

	// PVCProvisionerAnnotationKey is the PVC annotation that stores the PVC provisioner name.
	PVCProvisionerAnnotationKey = "volume.beta.kubernetes.io/storage-provisioner"
)

// IsStorageOS returns true if the Node or PVC provided is a StorageOS
// provisioner or has been provisioned by StorageOS.
func IsStorageOS(obj client.Object) (bool, error) {
	switch obj.GetObjectKind().GroupVersionKind().Kind {
	case "Node":
		return IsStorageOSNode(obj)
	case "PersistentVolumeClaim":
		return IsStorageOSPVC(obj), nil
	case "Volume":
		// Use IsStorageOSVolume() directly.
		return false, ErrUnsupportedKind
	default:
		return false, ErrUnsupportedKind
	}
}

// IsStorageOSNode returns true if the object has the StorageOS CSI driver
// annotation.  This will only be set on Node objects.
//
// The annotation is added by the drivers' node registrar, so should only be
// present on node objects that have run StorageOS.  Once added, there is
// currently no code to remove it from a node.
func IsStorageOSNode(obj client.Object) (bool, error) {
	annotations := obj.GetAnnotations()
	drivers, ok := annotations[NodeDriverAnnotationKey]
	if !ok {
		return false, nil
	}
	driversMap := map[string]string{}
	if err := json.Unmarshal([]byte(drivers), &driversMap); err != nil {
		return false, err
	}
	if _, ok := driversMap[DriverName]; ok {
		return true, nil
	}
	return false, nil
}

// IsStorageOSPVC returns true if the object has the StorageOS CSI driver
// set as the storage provisioner.  This will only be set on PVCs.
//
// The annotation is added by Kubernetes core during the PVC provisioning
// workflow, immediately after the provisioner is determined from the
// StorageClass.  It is only set on dynamically-provisioned PVCs (which is ok).
func IsStorageOSPVC(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	provisioner, ok := annotations[PVCProvisionerAnnotationKey]
	if ok {
		return provisioner == DriverName
	}
	return false
}

// IsProvisionedPVC returns true if the PVC was provided by one of
// the given provisioners.
func IsProvisionedPVC(k8s client.Client, pvc corev1.PersistentVolumeClaim, namespace string, provisioners ...string) (bool, error) {
	// Get the StorageClass that provisioned the volume.
	sc, err := StorageClassForPVC(k8s, &pvc)
	if err != nil {
		return false, err
	}

	// Check if the StorageClass provisioner matches with any of the provided
	// provisioners.
	for _, provisioner := range provisioners {
		if sc.Provisioner == provisioner {
			// This is a managed volume.
			return true, nil
		}
	}

	return false, nil
}

// IsStorageOSVolume returns true if the volume's PVC was provisioned by
// StorageOS.  The namespace of the Pod/PVC must be provided.
func IsStorageOSVolume(k8s client.Client, volume corev1.Volume, namespace string) (bool, error) {
	return IsProvisionedVolume(k8s, volume, namespace, DriverName)
}

// IsProvisionedVolume returns true if the volume's PVC was provided by one of
// the given provisioners.
func IsProvisionedVolume(k8s client.Client, volume corev1.Volume, namespace string, provisioners ...string) (bool, error) {
	// Ensure that the volume has a claim.
	if volume.PersistentVolumeClaim == nil {
		return false, nil
	}

	// Get the PersistentVolumeClaim object.
	pvc := &corev1.PersistentVolumeClaim{}
	key := types.NamespacedName{
		Name:      volume.PersistentVolumeClaim.ClaimName,
		Namespace: namespace,
	}
	if err := k8s.Get(context.Background(), key, pvc); err != nil {
		return false, errors.Wrap(err, "failed to get PVC")
	}

	return IsProvisionedPVC(k8s, *pvc, namespace, provisioners...)
}
