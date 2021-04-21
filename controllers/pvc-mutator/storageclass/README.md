# PVC StorageClass mutator

The PVC StorageClass mutator tries to detect the StorageClass of the newly created PVC.
It sets the `UID` of the StorageClass as an annotation on PVC.

## Trigger

Only PVCs that will be provisioned by StorageOS are candidates for mutation.

## Tunables

There are currently no tunable flags for StorageClass Parameter Sync.
