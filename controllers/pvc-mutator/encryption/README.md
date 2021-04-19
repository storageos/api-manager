# PVC Encryption Mutator

The PVC Encryption Mutator ensures that PVCs that have requested encryption have
a valid configuration, generating encryption keys if needed.

Users may request volume encryption by setting the
`storageos.com/encrption=true` label on the PVC.  PVCs that don't have this
label will be ignored and left unchanged.

When encryption has been requested, the mutator checks to see whether encryption
keys already exist, and if not, generates them.  Keys are stored as Kubernetes
Secrets.

## Encryption key secret

The reference to the secret containing the volume encryption key is set as
annotations on the PVC:

- `storageos.com/encryption-secret-name`: The name of the secret containing the
  volume encryption key.
- `storageos.com/encryption-secret-namespace`: The namespace of the secret
  containing the volume encryption key.  This must match the PVC namespace.

If the secret reference annotations are not set by the user, then they will be
generated and set by the api-manager.

Auto-generated secrets will be stored in the PVC namespace.  The secret name
will be a concatenation of `key`  and the PVC name, separated by `.`.

If the PVC was created with the secret reference annotations already present,
they will be used instead.  If they point to a secret that does not exist, a new
key will be generated and stored there.  If the api-manager does not have
permission to write the secret, or the namespace does not exist, then PVC
creation will fail.

## RBAC

In the default configuration, api-manager requires full access to secrets in the
namespaces that PVC will be created in.  This is enabled by default.

## Trigger

Only PVCs that will be provisioned by StorageOS and have the label
`storageos.com/encryption=true` are candidates for mutation.

## Garbage collection

Encryption key secrets must be manually deleted after they are no longer
required.  This may be automated in the future.

## Tunables

There are currently no tunable flags for PVC Encryption.
