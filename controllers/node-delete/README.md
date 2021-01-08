# Node Delete Controller

The Node Delete Controller is responsible for removing nodes from the StorageOS
cluster when the node has been removed from Kubernetes.

The intention is to allow StorageOS to adapt to dynamic environments where nodes
are added and removed regularly, such as when spot instances are used.

Removing nodes when they are no longer required reduces load as StorageOS will
no longer check for the node recovering.

## Trigger

The controller reconcile will trigger on any Kubernetes Node delete event where
the node has the StorageOS CSI driver annotation.

The annotation is added by the CSI node driver registrar when StorageOS starts
on the node.  Once added, it is not removed.

## Reconcile

When a Kubernetes node with the StorageOS CSI driver annotation is deleted, a
request is made to the StorageOS API to remove the node.  The StorageOS API will
only allow the delete to succeed if:

- The node has been marked offline (typically discovered 20-30 seconds after it
  goes off the network).
- The node lock has expired in etcd.  Locks expire after 30 seconds.
- The node no longer holds any master deployments.  Normally the master
  deployment would fail over to another node after it was marked offline, but in
  the case where no replicas have been configured, blocking the delete allows
  the data to be recovered.  To proceed with the node removal, delete the volume
  first.

If the delete request failed, it will be requeued and retried after a backoff
period.  

Typically there will be multiple "node still in use" log entries before
the node container is shutdown and StorageOS detects that it is offline.  There
may also be one or more "node lock has not yet expired" messages before the node
is finally removed.

If the node was not found because it has already been deleted, the delete
request will be considered successful.

## Garbage Collection

In case a node delete event was missed during a restart or outage, a garbage
collection runs periodically.  It compares the list of nodes known to StorageOS,
and removes any that are no longer known to Kubernetes.

Garbage collection is run every hour by default (configurable via the
`-node-delete-gc-interval` flag).
