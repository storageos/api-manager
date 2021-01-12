package nodelabel

import (
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/storageos/api-manager/internal/pkg/annotation"
	"github.com/storageos/api-manager/internal/pkg/predicate"
)

// Predicate filters events before enqueuing the keys.  Ignore all but Update
// events, and then filter out events from non-StorageOS nodes.
//
// Nodes added to the cluster will need to wait for the next label resync to
// have labels from the initial object applied.  We can't react to create events
// as the node will not exist in StorageOS yet.
//
// TODO(sc) consider reacting to Annotation changes, and triggering a resync
// when the StorageOS CSI driver annotation in added.
type Predicate struct {
	predicate.IgnoreFuncs
	log logr.Logger
}

// Update determines whether an object update should trigger a reconcile.
func (p Predicate) Update(e event.UpdateEvent) bool {
	// Ignore nodes that haven't run StorageOS.
	found, err := annotation.StorageOSCSIDriver(e.ObjectNew)
	if err != nil {
		p.log.Error(err, "failed to process node annotations", "node", e.ObjectNew.GetName())
	}
	if !found {
		return false
	}

	// Reconcile only on label changes.
	if !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) {
		return true
	}
	return false
}
