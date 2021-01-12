# Node Label Sync Controller

The Node Label Sync Controller is responsible for syncing labels set on
Kubernetes nodes to the StorageOS node objects.

Some StorageOS functionality, such as marking a node as "compute only", is done
by setting the `storageos.com/computeonly=true` label on a Kubernetes node.
This controller ensures that the behaviour is applied to the StorageOS node.

## Trigger

The controller reconcile will trigger on any Kubernetes Node label update event
where the node has the StorageOS CSI driver annotation.

The annotation is added by the CSI node driver registrar when StorageOS starts
on the node.  Once added, it is not removed.

## Reconcile

When the labels on a Kubernetes node with the StorageOS CSI driver annotation is
updated, a request is made to the StorageOS API to re-apply the labels to the
StorageOS node.

Labels prefixed with `storageos.com/` have special meaning, and will likely be
applied with a discrete call to the StorageOS API.  This ensures that the
behaviour can be applied in a strongly-consistent manner or return an error.

Remaining labels without the `storageos.com/` prefix will be applied as a single
API call.  They have no internal meaning to StorageOS but they can be used to
influence placement decisions.

If a nodel label sync fails, it will be requeued and retried after a backoff
period.  It is possible that the application of only a partial set of labels
will succeed.  If StorageOS can't apply a certain behaviour change (for example,
if the change would result in a volume going offline), then only that behaviour
change would fail and the remaining changes would be attempted.  If any change
fails, the whole set of labels will be retried until they all succeed.

## Resync

In case a node label update event was missed during a restart or outage, a
resync runs periodically.  It re-applies the set of Kubernetes node labels to
StorageOS nodes.

This is also used when a Kubernetes node is added to the cluster.  Initially
StorageOS will not be running on the node so it is not possible to sync labels
from the node create event.  Instead, the labels will be synchronized on the
next resync.

Node label resync is run every hour by default (configurable via the
`--node-label-resync-interval` flag).
