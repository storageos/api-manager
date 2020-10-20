# Shared Volume Controller

The Shared Volume controller is responsible for creating Kubernetes Services and
Endpoint resources for attached shared volumes and then publishing the Service's
endpoint to the `storageos.com/nfs/mount-endpoint` label on the volume.

## Create and Publish Shared Volume

When the StorageOS control plane receives a CSI `CreateVolume` request for a
`ReadWriteMany` (RWX) volume, it creates a standard volume.  All StorageOS
volumes support both `SINGLE_NODE_WRITER` and `MULTI_NODE_MULTI_WRITER` CSI
access modes.

Only when the control plane receives a CSI `ControllerPublishVolume` with
`VolumeCapability` set to the `MULTI_NODE_MULTI_WRITER` access mode does it
attach the volume as shared.  When attached, the volume will have:

- `attachmentType` set to `nfs`.
- `nfs.serviceEndpoint` set to the address at which the NFS server is bound.

These steps happen without involvement from the Shared Volume controller.

## Shared Volume Control Loop

The Shared Volume controller periodically polls the StorageOS control plane API
to retrieve a list of volumes.  The poll interval defaults to `5s` and is
tunable via the `-api-poll-interval` flag.

Only volumes that have all of the follwing will be considered:

- `nfs.serviceEndpoint` set to a valid `<ip>:<port>`.
- `csi.storage.k8s.io/pv/name` label set.
- `csi.storage.k8s.io/pvc/name` label set.
- `csi.storage.k8s.io/pvc/namespace` label set.
- PVC matching labels above.

Volumes without all of the above will be silently ignored.

## Shared Volume cache

The Shared Volume controller maintains a cache of Shared Volumes, primarily to
reduce the risk of throttling from the Kubernetes API server when verifying the
volume's required resources.

Volumes are only added to the cache once its required resources have been
verified to be present.

Cached volumes expire after the `-cache-expiry-interval` (default `1m`), which
is set per-volume when it is added to the cache.  Increasing the interval will
increase the time taken to recreate k8s resources if they were deleted manually.

When a volume's cache entry expires, it will be treated as a new volume and its
Service and Endpoints will be created if missing.

If a volume is in the cache and has not expired, it is compared with the volume
returned from the API.  If there is a difference, the Service and Endpoints will
be re-evaluated immediately.

## Kubernetes Resource Evaluation

Shared Volumes must have a Service created with a `ClusterIP` that does not
change for the lifetime of the PVC.  The `ClusterIP` is combined with a static
port (`2049`, the default NFS port), and set as the
`storageos.com/nfs/mount-endpoint` label on the StorageOS volume.

An Endpoint must also exist with the same name and namespace as the Service,
with the target set to the NFS server endpoint as defined in
`nfs.serviceEndpoint`.

Since a Shared Volume is tied to a specific PVC, the Service and Endpoint both
use the PVC name and namespace.

Resources are checked for existence and equivalence before deciding whether a
create, update, or no action is required.  When a resource is created, it is
re-fetched before proceeding.  Since the resource may not appear in the k8s api
immediately, the resource is polled every `-k8s-create-poll-interval` (default
`1s`) for `-k8s-create-wait-duration` (default `20s`).

## Mount Endpoint Publishing

Only once the Kubernetes resources have been successfully re-evaluated does the
Service endpoint get published in `storageos.com/nfs/mount-endpoint` on the
StorageOS volume, and only if different from the existing value.

In normal operation the Service endpoint should not change - doing so will
invalidate client caches and lead to "Stale NFS filehandle" errors.  The only
likely cause would be in the Service was manually deleted.

## Mount Shared Volume

After the CSI `ControllerPublishVolume` succeeds, it's likely that
`NodePublishVolume` will be called immediately to mount the volume into the
application container.  This will not succeed until `storageos.com/nfs/mount-endpoint`
has been set by the Shared Volume Controller.  The control plane uses the mount
endpoint to remotely (or locally) mount the shared volume.

## Volume Failover

When a StorageOS master volume fails over to another node, the NFS service gets
restarted on that node and the `nfs.serviceEndpoint` is updated to reflect the
new endpoint.

The shared volume control loop will either:

- ignore the volume if the volume no longer has `nfs.serviceEndpoint` set.
- see that the cached volume no longer matches due to a different
  `nfs.serviceEndpoint` and will trigger a resource re-evaluation.

The Service will be updated with the new target port and the Endpoint will be
updated with the new address and port.

During failover and update, the Service endpoint (`<ClusterIP>:2049`) does not
change but it will not respond until the Endpoint has been updated.

## Garbage Collection

Services and Endpoints are automatically removed when the PVC is deleted.  The
PVC is set as the owner of the service, and the Kubernetes garbage collector
will delete it and the Endpoint, which is automatically associated with the Service.

## Prometheus Metrics

The following metrics are collected:

- `storageos_api_helper_duration_seconds` Distribution of the length of time api
  helpers take to complete.  API helper name set as `function` label.
- `storageos_api_helper_total` Number of api helper calls, partitioned by
  function name and error string.
- `storageos_api_in_flight_requests` A gauge of in-flight requests for the api client.
- `storageos_api_request_duration_seconds` A histogram of request latencies,
  partitioned by HTTP request method.
- `storageos_api_requests_total` A counter for requests from the api client,
  partitioned by HTTP request method and response code.
- `storageos_shared_volume_reconcile_duration_seconds` Distribution of the
  length of time taken to reconcile all shared volumes.
