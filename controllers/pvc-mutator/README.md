# PVC Mutator Admission Controller

The PVC Mutator is a mutating admission controller that modifies
PersistentVolumeClaims during the create process.

## Mutators

The PVC Mutator can run multiple mutation functions, each performing a different
task:

- [Encryption key generator](/controllers/pvc-mutator/encryption/README.md):
  ensures that PVCs that have requested encryption have a valid configuration,
  generating encryption keys if needed.

- [StorageClass to annotation](/controllers/pvc-mutator/storageclass/README.md):
  ensures that StorageOS related PVCs have them StorageClass as an annotation.

## Tunables

There are currently no tunable flags for the PVC Mutator.
