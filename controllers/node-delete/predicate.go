package nodedelete

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/storageos/api-manager/internal/pkg/predicate"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
)

// Predicate filters events before enqueuing the keys.  Ignore all but Delete
// events, and then filter out events from non-StorageOS nodes.
type Predicate struct {
	predicate.IgnoreFuncs
	log logr.Logger
}

// Delete determines whether an object delete should trigger a reconcile.
func (p Predicate) Delete(e event.DeleteEvent) bool {
	// Ignore objects without the StorageOS CSI driver annotation.
	found, err := provisioner.IsStorageOSNode(e.Object)
	if err != nil {
		p.log.Error(err, "failed to process node annotations", "node", e.Object.GetName())
	}
	return found
}
