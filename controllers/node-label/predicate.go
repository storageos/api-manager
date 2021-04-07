package nodelabel

import (
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/storageos/api-manager/internal/pkg/predicate"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
)

// Predicate filters events before enqueuing the keys.  Ignore all but Update
// events, and then filter out events from non-StorageOS nodes.  Trigger a
// resync when labels have changed.
//
// Nodes added to the cluster will not immediately be added to StorageOS, so we
// can't react to node create events.  Instead, trigger a resync when the
// StorageOS CSI driver annotation has been added, indicating that the node is
// known to StorageOS and can receive label updates.
type Predicate struct {
	predicate.IgnoreFuncs
	log logr.Logger
}

// Update determines whether an object update should trigger a reconcile.
func (p Predicate) Update(e event.UpdateEvent) bool {
	// Ignore nodes that haven't run StorageOS.
	isStorageOS, err := provisioner.IsStorageOSNode(e.ObjectNew)
	if err != nil {
		p.log.Error(err, "failed to process current node annotations", "node", e.ObjectNew.GetName())
	}
	if !isStorageOS {
		return false
	}

	// Reconcile if the StorageOS CSI annotation was just added.
	wasStorageOS, err := provisioner.IsStorageOSNode(e.ObjectOld)
	if err != nil {
		p.log.Error(err, "failed to process previous node annotations", "node", e.ObjectOld.GetName())
	}
	if !wasStorageOS {
		return true
	}

	// Otherwise reconcile on label changes.
	if !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) {
		return true
	}
	return false
}
