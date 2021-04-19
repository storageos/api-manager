package pvclabel

import (
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/storageos/api-manager/internal/pkg/predicate"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
)

// Predicate filters events before enqueuing the keys.  Ignore all but Update
// events, and then filter out events from non-StorageOS PVCs.  Trigger a
// resync when labels have changed.
//
// We don't need to react to PVC create events as PVC labels will be set in the
// CSI create volume request as params.  This is a customization made to the CSI
// Provisioner.
type Predicate struct {
	predicate.IgnoreFuncs
	log logr.Logger
}

// Update determines whether an object update should trigger a reconcile.
func (p Predicate) Update(e event.UpdateEvent) bool {
	// Ignore PVCs that aren't provisvioned by StorageOS.
	if !provisioner.HasStorageOSAnnotation(e.ObjectNew) {
		return false
	}

	// Otherwise reconcile on label changes.
	if !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) {
		return true
	}
	return false
}
