# Namespace Delete Controller

The Namespace Delete Controller is responsible for removing namespaces from the
StorageOS cluster when the namespace has been removed from Kubernetes.

When a StorageOS-provisioned PVC is created in a Kubernetes namespace, if the
namespace does not already exist in the StorageOS cluster then it is created and
the StorageOS volume is created within it.

The opposite is not true - the StorageOS control plane does not remove
namespaces after the last volume has been removed from it, as it can't tell
whether a new volume will be provisioned to it.  This can lead to multiple empty
namespaces remaining in the cluster, unless they are deleted manually using the
UI or CLI.

If the Kubernetes namespace is deleted, then we can safely assume that the
namespace is no longer needed, and that it will be re-created if a new PVC is
provisioned to it.  Only empty namespaces will be deleted.

## Trigger

The controller reconcile will trigger on any Kubernetes Namespace delete event.

## Reconcile

When a Kubernetes namespace is deleted, a request is made to the StorageOS API
to remove the namespace.  The StorageOS API will only allow the delete to
succeed if the namespace is empty and does not contain any volumes.

If the delete request failed, it will be requeued and retried after a backoff
period.

If the namespace was not found, either because it was already deleted or it
never had a PVC provisioned by StorageOS, the delete request will be considered
successful.

## Garbage Collection

In case a namespace delete event was missed during a restart or outage, a
garbage collection runs periodically.  It compares the list of namespaces known
to StorageOS, and removes any that are no longer known to Kubernetes.

Garbage collection is run every hour by default (configurable via the
`-namespace-delete-gc-interval` flag).  It can be disabled by setting
`-namespace-delete-gc-interval` to `0s`.

Garbage collection is run on startup after a delay defined by the
`-namespace-delete-gc-delay` flag.
