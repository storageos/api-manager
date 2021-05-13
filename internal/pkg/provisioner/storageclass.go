package provisioner

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// pvcStorageClassKey is the annotation used to refer to the StorageClass when
	// the PVC storageClassName wasn't used.  This is now deprecated but should
	// still be checked as k8s still supports it.
	pvcStorageClassKey = "volume.beta.kubernetes.io/storage-class"

	// DefaultStorageClassKey is the annotation used to denote whether a
	// StorageClass is the cluster default.
	DefaultStorageClassKey = "storageclass.kubernetes.io/is-default-class"

	// StorageClassUUIDAnnotationKey is the annotation on the PVC that stores
	// the UUID of the StorageClass that was used to provision it.  It is used
	// to detect when the StorageClass was deleted and re-created with the same
	// name.
	StorageClassUUIDAnnotationKey = "storageos.com/storageclass"

	// reservedParamPrefix is the prefix used to determine whether a parameter
	// should be considered "reserved" by StorageOS.
	reservedParamPrefix = "storageos.com/"
)

// StorageClassForPVC returns the StorageClass of the PVC.  If no StorageClass
// was specified, returns the cluster default if set.
func StorageClassForPVC(k8s client.Client, pvc *corev1.PersistentVolumeClaim) (*storagev1.StorageClass, error) {
	name := PVCStorageClassName(pvc)
	if name == "" {
		return DefaultStorageClass(k8s)
	}
	return StorageClass(k8s, name)
}

// StorageClass returns the StorageClass matching the name.  If no name was
// specified, the default StorageClass (if any) is returned instead.
func StorageClass(k8s client.Client, name string) (*storagev1.StorageClass, error) {
	if name == "" {
		return DefaultStorageClass(k8s)
	}
	key := types.NamespacedName{
		Name: name,
	}
	sc := &storagev1.StorageClass{}
	if err := k8s.Get(context.Background(), key, sc); err != nil {
		return nil, fmt.Errorf("failed to get StorageClass: %w", err)
	}
	return sc, nil
}

// DefaultStorageClass returns the default StorageClass, if any.
func DefaultStorageClass(k8s client.Client) (*storagev1.StorageClass, error) {
	scList := &storagev1.StorageClassList{}
	if err := k8s.List(context.Background(), scList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to get StorageClasses: %v", err)
	}
	for _, sc := range scList.Items {
		if val, ok := sc.Annotations[DefaultStorageClassKey]; ok && val == "true" {
			return &sc, nil
		}
	}
	return nil, fmt.Errorf("default StorageClass not found")
}

// PVCStorageClassName returns the PVC provisioner name.
func PVCStorageClassName(pvc *corev1.PersistentVolumeClaim) string {
	// The beta annotation should still be supported since even latest versions
	// of Kubernetes still allow it.
	if pvc.Spec.StorageClassName != nil && len(*pvc.Spec.StorageClassName) > 0 {
		return *pvc.Spec.StorageClassName
	}
	if val, ok := pvc.Annotations[pvcStorageClassKey]; ok {
		return val
	}
	return ""
}

// StorageClassReservedParams returns a map of StorageClass parameters that are
// reserved for StorageOS.  These are typically feature defaults for any volumes
// provisioned by the StorageClass.
func StorageClassReservedParams(sc *storagev1.StorageClass) map[string]string {
	reserved := make(map[string]string)
	for k, v := range sc.Parameters {
		if strings.HasPrefix(k, reservedParamPrefix) {
			reserved[k] = v
		}
	}
	return reserved
}

// ValidateOrSetStorageClassUID returns true if the StorageClass annotation on
// the PVC object matches the uid passed in or the StorageClass is younger than PVC.
//
// If the annotation does not exist and the StorageClass is younger than PVC,
// it sets the uid passed in as the new StorageClass annotation.
func ValidateOrSetStorageClassUID(ctx context.Context, k8s client.Client, sc client.Object, pvc client.Object) (bool, error) {
	provisionedUID := pvc.GetAnnotations()[StorageClassUUIDAnnotationKey]
	if provisionedUID != "" {
		// If UID of StorageClass and PVC are different, the synchronization isn't safe.
		return string(sc.GetUID()) == provisionedUID, nil
	}

	// If StorageClass is younger than the PVC itself, the synchronization isn't safe.
	if sc.GetCreationTimestamp().Unix() > pvc.GetCreationTimestamp().Unix() {
		return false, nil
	}

	// Convert client object to PersistentVolumeClaim.
	var v1Pvc corev1.PersistentVolumeClaim
	if err := k8s.Get(ctx, client.ObjectKeyFromObject(pvc), &v1Pvc); err != nil {
		return false, err
	}

	// Annotation not set, set it.
	v1Pvc.Annotations[StorageClassUUIDAnnotationKey] = string(sc.GetUID())
	if err := k8s.Update(ctx, &v1Pvc, &client.UpdateOptions{}); err != nil {
		return false, err
	}

	return true, nil
}
