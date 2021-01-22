package annotation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ProvisionerAnnotationKey is the annotation that stores the PVC provisioner name.
	ProvisionerAnnotationKey = "volume.beta.kubernetes.io/storage-provisioner"
)

// StorageOSProvisioned returns true if the object has the StorageOS CSI driver
// set as the storage provisioner.  This will only be set on PVCs.
//
// The annotation is added by Kubernetes core during the PVC provisioning
// workflow, immediately after the provisioner is determined from the
// StorageClass.  It is only set on dynamically-provisioned PVCs (which is ok).
func StorageOSProvisioned(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	provisioner, ok := annotations[ProvisionerAnnotationKey]
	if ok {
		return provisioner == DriverName
	}
	return false
}
