// Package annotation contains helers for working with Kubernetes Annotations.
package annotation

import (
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DriverName is the name of the StorageOS CSI driver.
	DriverName = "csi.storageos.com"

	// DriverAnnotationKey is the annotation that stores the registered CSI
	// drivers.
	DriverAnnotationKey = "csi.volume.kubernetes.io/nodeid"
)

// StorageOSCSIDriver returns true if the object has the StorageOS CSI driver
// annotation.  This will only be set on Node objects.
//
// The annotation is added by the drivers' node registrar, so should only be
// present on node objects that have run StorageOS.  Once added, there is
// currently no code to remove it from a node.
func StorageOSCSIDriver(obj client.Object) (bool, error) {
	annotations := obj.GetAnnotations()
	drivers, ok := annotations[DriverAnnotationKey]
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
