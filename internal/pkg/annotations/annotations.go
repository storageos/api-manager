// Package annotation contains helers for working with Kubernetes Annotations.
package annotations

import "encoding/json"

const (
	// DriverName is the name of the StorageOS CSI driver.
	DriverName = "csi.storageos.com"

	// DriverAnnotationKey is the annotation that stores the registered CSI
	// drivers.
	DriverAnnotationKey = "csi.volume.kubernetes.io/nodeid"
)

// IncludesStorageOSDriver returns true if the annotations include the StorageOS
// driver annotation.
//
// The annotation is added by the drivers' node registrar, so should only be
// present on nodes that have run StorageOS.  Once added, there is currently no
// code to remove it from a node.
func IncludesStorageOSDriver(annotations map[string]string) (bool, error) {
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
