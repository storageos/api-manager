package nsdelete

import (
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/storageos/api-manager/internal/pkg/predicate"
)

// Predicate filters events before enqueuing the keys.  Ignore all but Delete
// events.
type Predicate struct {
	predicate.IgnoreFuncs
}

// Delete determines whether an object delete should trigger a reconcile.
func (p Predicate) Delete(e event.DeleteEvent) bool {
	return true
}
