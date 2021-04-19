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

	// defaultStorageClassKey is the annotation used to denote whether a
	// StorageClass is the cluster default.
	defaultStorageClassKey = "storageclass.kubernetes.io/is-default-class"

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
		if val, ok := sc.Annotations[defaultStorageClassKey]; ok && val == "true" {
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
// the PVC object matches the uid passed in.
//
// If the annotation does not exist, it sets the uid passed in as the new
// StorageClass annotation.
func ValidateOrSetStorageClassUID(ctx context.Context, k8s client.Client, uid types.UID, obj client.Object) (bool, error) {
	provisionedUID := obj.GetAnnotations()[StorageClassUUIDAnnotationKey]
	if provisionedUID != "" {
		return string(uid) == provisionedUID, nil
	}

	// Annotation not set, set it.
	if err := SetStorageClassUIDAnnotation(ctx, k8s, uid, obj); err != nil {
		return false, err
	}
	return true, nil
}

// SetStorageClassUIDAnnotation adds the StorageClass annotation to a PVC object.
func SetStorageClassUIDAnnotation(ctx context.Context, k8s client.Client, uid types.UID, obj client.Object) error {
	var pvc corev1.PersistentVolumeClaim
	if err := k8s.Get(ctx, client.ObjectKeyFromObject(obj), &pvc); err != nil {
		return err
	}
	pvc.Annotations[StorageClassUUIDAnnotationKey] = string(uid)
	return k8s.Update(ctx, &pvc, &client.UpdateOptions{})
}
