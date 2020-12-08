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

When a Kubernetes node with the StorageOS driver annotation is deleted, a
request is made to the StorageOS API to remove the node.  The StorageOS API will
only allow the delete to succeed if the node has been marked offline (typically
discovered 20-30 seconds after it goes off the network).

If the delete request failed, it will be requeued and retried after a backoff
period.

If the node was not found because it has already been deleted, the delete
request will be considered successful.
