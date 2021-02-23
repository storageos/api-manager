package provisioner

import (
	"context"
	"fmt"

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
)

// StorageClassForPVC returns the StorageClass of the PVC.  If no StorageClass
// was specified, returns the cluster default if set.
func StorageClassForPVC(k8s client.Client, pvc *corev1.PersistentVolumeClaim) (*storagev1.StorageClass, error) {
	scName := PVCStorageClassName(pvc)
	if scName == "" {
		return DefaultStorageClass(k8s)
	}
	sc := &storagev1.StorageClass{}
	scNSName := types.NamespacedName{
		Name: scName,
	}
	if err := k8s.Get(context.Background(), scNSName, sc); err != nil {
		return nil, fmt.Errorf("failed to get StorageClass: %v", err)
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
