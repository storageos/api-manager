# StorageClass Parameter Sync

The StorageClass Parameter Sync mutator copies StorageOS-specific parameters from the StorageClass to the PVC as labels.
This ensures that the defaults set in the StorageClass aren't overwritten by the PVC Label Sync controller.

The StorageClass Parameter Sync mutator will not overwrite existing PVC labels.

## Trigger

Only PVCs that will be provisioned by StorageOS are candidates for mutation.

## Tunables

There are currently no tunable flags for StorageClass Parameter Sync.
