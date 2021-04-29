# Fencing Controller

The Fencing Controller can be used to enable fast failover of workloads when a
node goes offline.  This is particularly useful when the workload is deployed
using a StatefulSet.

To protect data integrity, Kubernetes guarantees that there will never be more
than one instance of a StatefulSet Pod running at a time.  It assumes that when a node is
determined to be offline it may still be running but partitioned from the
network and still running the workload.  Since Kubernetes is unable to verify that the
Pod has been stopped it errs on the side of caution and does not allow a
replacement to start on another node.

For this reason, Kubernetes requires manual intervention to initiate a failover
of a StatefulSet Pod.

Since StorageOS is able to determine when a node is no longer able to access a
volume and has protection to ensure that a partitioned or formerly partitioned
node can not continue to write data, it can work with Kubernetes to perform
safe, fast failovers of Pods, including those running in StatefulSets.

When StorageOS detects that a node has gone offline or become partitioned, it
marks the node offline and performs volume failover operations.

The fencing controller watches for node failures and determines if there are any
Pods assigned to the node that have the `storageos.com/fenced=true` label set
and PVCs backed by StorageOS volumes.

When a Pod has StorageOS volumes and if they are all healthy, the fencing
controller will delete the Pod to allow it to be rescheduled on another node.
It also deletes the VolumeAtachments for the corresponding volumes so that they
can be immediately attached to the new node.

No changes are made to Pods that have StorageOS volumes that are unhealthy.
This is likely where a volume was configured to not have any replicas, and the
node with the single copy of the data is offline.  In this case it is better to
wait for the Node to recover.

Fencing works with both dynamically provisioned PVCs and PVCs referencing
pre-provisioned volumes.

The fencing feature is opt-in and Pods must have the `storageos.com/fenced=true`
label set to enable fast failover.

## Trigger

The controller reconcile will trigger on any StorageOS node in unhealthy state.
StorageOS nodes are polled every `5s`, configurable with the
`-node-poll-interval` flag.  This determines how quickly the fencing controller
can react to node failures.

All nodes are also re-evaluated for fencing every `1h`, configurable with the
`-node-expiry-interval` flag.  When nodes expire from the cache their status is
re-evaluated. If the node is unhealthy, the controller reconcile will be
triggered.

A side-effect of the cache expiry is that if there were Pods on the failed node
that had unhealthy volumes and thus ignored during the initial fencing
operation, they may now be processed.  If the volumes have recovered since the
initial fencing attempt, then fencing will proceed when the node is processed
again due to the cache expiry.  This behaviour may change or be removed in the
future, depending on feedback.

## Reconcile

When a StorageOS node has been detected offline, the fencing controller performs
the following actions:

- Lists all Pods running on the failed node.
- For each Pod:

  - Verify that the Pod has the `storageos.com/fenced=true` label set, otherwise
    ignore the Pod.
  - Retrieves list of StorageOS PVCs for the Pod.  Skips Pods that have no
    StorageOS PVCs.
  - Verify that the StorageOS volume backing each of the Pod's StorageOS PVCs is
    healthy. If not, skip the Pod.
  - Delete the Pod.
  - Delete the VolumeAttachments for the StorageOS PVCs.

- The fencing operation for a node has a timeout of `25s`, configurable with the
  `-node-fencer-timeout` flag.  When the timeout is exceeded, the controller
  will log an error.
- If any errors were encountered during the fencing operation, and the timeout
  hasn't been reached, the operation will be retried after a `5s` delay.  The
  delay is configurable with the `-node-fencer-retry-interval` flag.
- Once the fencing operation has completed, the node will not re-evaluated again
  until its status changes to healthy and unhealthy again, or it has expired
  from the cache.
