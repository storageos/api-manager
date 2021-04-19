# PVC Label Sync Controller

The PVC Label Sync Controller is responsible for syncing labels set on
Kubernetes PVCs to the StorageOS volume objects.

Labels are initially set at creation time by a custom [CSI Provisioner], that
adds PVC labels to the CSI CreateVolume parameters.  These are added on top of
any default parameters set in the StorageClass parameters.

This controller ensures that any PVC label changes are applied.

Some StorageOS functionality, such as setting the number of desired replicas, is
done by setting or changing the `storageos.com/replicas=N` label on a Kubernetes
PVC (where N is from 0-6).  This controller ensures that the behaviour is
applied to the StorageOS volume after it has been created.

Other labels, such as `storageos.com/nocache` and `storageos.com/nocompress` can
only be set when the volume is created, so the PVC Label Sync Controller ignores
them.

See [StorageOS Feature Labels] for more information.

# StorageClass Defaults

Cluster administrators may set defaults for volumes by setting feature labels as
parameters in the StorageClass.  The PVC Label Sync Controller will load the
StorageClass parameters prior to applying any label changes to ensure that they
are taken into account and not removed.

The controller needs to ensure that the defaults set in the StorageClass have
not changed since the volume was provisioned.  Otherwise a change to a feature
label in the StorageClass would get immediately applied to all volumes, which
would not be the expected behaviour.

Since StorageClasses are immutable, to change a parameter requires deleting and
recreating the StorageClass.  To detect this, when the PVC is created, the UID
of the StorageClass is set in the `storageos.com/storageclass` annotation on the
PVC by the [PVC StorageClass Annotation
Mutator](controllers/pvc-mutator/storageclass/README.md).  The PVC Label Sync
Controller verifies that the current StorageClass UID matches.  If not, labels
are not synchronised for the PVC.

To re-enable PVC label sync when there is a StorageClass UID mismatch, manually confirm
that any StorageClass parameter changes are intended to be applied, then remove
the PVC StorageClass annotation.

## Trigger

The controller reconcile will trigger on any Kubernetes PVC label update event
where the PVC has the StorageOS CSI driver listed in the storage provisioner
annotation.  Specifically, PVCs must have the annotation:

```yaml
volume.beta.kubernetes.io/storage-provisioner: csi.storageos.com
```

The annotation is added by Kubernetes when the PVC is evaluated to determine the
provisioner to use.  This is determined by the PVC's StorageClassName parameter,
or if not set, the default StorageClass.  Once set it is not changed or removed.

## Reconcile

When the labels on a Kubernetes PVC with the StorageOS provisioner annotation is
updated, a request is made to the StorageOS API to re-apply the labels to the
corresponding StorageOS volume.

Labels prefixed with `storageos.com/` have special meaning, and will likely be
applied with a discrete call to the StorageOS API.  This ensures that the
behaviour can be applied in a strongly-consistent manner or return an error.

Remaining labels without the `storageos.com/` prefix will be applied as a single
API call.  They have no internal meaning to StorageOS but they can be used to
influence placement decisions.

If a PVC label sync fails, it will be requeued and retried after a backoff
period.  It is possible that the application of only a partial set of labels
will succeed.  If StorageOS can't apply a certain behaviour change (for example,
if the change would result in a volume going offline), then only that behaviour
change would fail and the remaining changes would be attempted.  If any change
fails, the whole set of labels will be retried until they all succeed.

## Resync

In case a PVC label update event was missed during a restart or outage, a
resync runs periodically.  It re-applies the set of Kubernetes PVC labels to
StorageOS volumes.

PVC label resync is run every hour by default (configurable via the
`-pvc-label-resync-interval` flag).  It can be disabled by setting
`-pvc-label-resync-interval` to `0s`.

Resync is run on startup after a delay defined by the
`-pvc-label-resync-delay` flag.

[CSI Provisioner]: https://github.com/storageos/external-provisioner/tree/53f0949-patched
[StorageOS Feature Labels]: https://docs.storageos.com/docs/reference/labels

## Disabling

The PVC Label Sync Controller can be disabled by setting the
`-enable-pvc-label-sync=false` flag.
