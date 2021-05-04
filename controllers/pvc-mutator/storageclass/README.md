# PVC StorageClass Annotation Mutator

The PVC StorageClass Annotation Mutator tries to detect the StorageClass of the
PVC being created and sets the StorageClass' UID in the
`storageos.com/storageclass` annotation on the PVC.

This allows StorageOS to detect whether the StorageClass parameters used to
create the volume may have been changed later.  Since StorageClasses are
immutable, deleting and recreating the StorageClass is the only way to change
parameters.

The [PVC Label Sync Controller](controllers/pvc-label/README.md) compares the
UID set in the annotation with the UID of the current StorageClass, and only
syncs labels if they match.

## Trigger

Only PVCs that will be provisioned by StorageOS are candidates for mutation.

## Failure Policy

Failure to set the StorageClass annotation should not cause the PVC creation to
fail.

## Tunables

There are currently no tunable flags for the PVC StorageClass Annotation
Mutator.
