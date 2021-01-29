# StorageOS API Manager

[![Test and build image](https://github.com/storageos/api-manager/workflows/Test%20and%20build%20image/badge.svg)](https://github.com/storageos/api-manager/actions?query=workflow%3A%22Test+and+build+image%22)

The StorageOS API Manager acts as a middle-man between various APIs.  It has all
the capabilites of a Kubernetes Operator and is also able to communicate with
the StorageOS control plane API.

## Controllers

Controllers are control loops that watch the state of the cluster.  Controllers
can be created to watch any object type, whether Kubernetes, StorageOS or
external, and to respond by making changes where needed.  Each controller tries
to move the current cluster state closer to the desired state.

### Shared Volume Controller

The shared volume controller watches the StorageOS control plane API for volumes
that are attached as shared volumes.  When a new shared volume attachment is
discovered, it finds the PVC for the volume and creates a Kubernetes Service for
it.  When the attachment address changes (e.g. after node failure), the Service
Endpoint will be updated, but the Service ClusterIP will remain the same.

See [Shared Volume Contoller](internal/controllers/sharedvolume/README.md) for
more detail.

### Node Delete Controller

The Node Delete Controller is responsible for removing nodes from the StorageOS
cluster when the node has been removed from Kubernetes.

See [Node Delete Contoller](controllers/node-delete/README.md) for more detail.

### Namespace Delete Controller

The Namespace Delete Controller is responsible for removing namespaces from the
StorageOS cluster when the namespace has been removed from Kubernetes.

See [Namespace Delete Controller](controllers/namespace-delete/README.md) for
more detail.

## Initialization

Startup blocks on obtaining a connection to the StorageOS control plane API,
which is retried every `-api-retry-interval` (default `5s`) until successful.
This allows the API Manager to start before the control plane API is ready,
avoiding a crash loop.

It does not participate in leader elections until it has a valid API connection.

### Leader Elections

At least two API Managers should run concurrently.  Leadership election ensures
that a single kubebuilder manager is active at a time.  Note that this does not
currently restrict multiple non-kubebuilder controllers (e.g. the Shared Volume
controller) from being active.

## Prometheus metrics

Prometheus metrics are available on `-metrics-addr` (default `:8080`).  See
controller documentation for specific stats.

## Installation

The API Manager is installed by the
[StorageOS cluster-operator](https://github.com/storageos/cluster-operator).  It
is not intended to be installed manually.

## Configuration

The following flags are supported:

```console
  -api-endpoint string
    	The StorageOS api endpoint address. (default "storageos")
  -api-poll-interval duration
    	Frequency of StorageOS api polling. (default 5s)
  -api-refresh-interval duration
    	Frequency of StorageOS api authentication token refresh. (default 1m0s)
  -api-retry-interval duration
    	Frequency of StorageOS api retries on failure. (default 5s)
  -api-secret-path string
    	Path where the StorageOS api secret is mounted.  The secret must have "username" and "password" set. (default "/etc/storageos/secrets/api")
  -cache-expiry-interval duration
    	Frequency of cached volume re-validation. (default 1m0s)
  -enable-leader-election
    	Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  -k8s-create-poll-interval duration
    	Frequency of Kubernetes api polling for new objects to appear once created. (default 1s)
  -k8s-create-wait-duration duration
    	Maximum time to wait for new Kubernetes objects to appear. (default 20s)
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -metrics-addr string
    	The address the metric endpoint binds to. (default ":8080")
  -namespace-delete-gc-delay duration
    	Startup delay of initial namespace garbage collection. (default 20s)
  -namespace-delete-gc-interval duration
    	Frequency of namespace garbage collection. (default 1h0m0s)
  -namespace-delete-workers int
    	Maximum concurrent namespace delete operations. (default 5)
  -node-delete-gc-delay duration
    	Startup delay of initial node garbage collection. (default 30s)
  -node-delete-gc-interval duration
    	Frequency of node garbage collection. (default 1h0m0s)
  -node-delete-workers int
    	Maximum concurrent node delete operations. (default 5)
  -node-label-resync-delay duration
    	Startup delay of initial node label resync. (default 10s)
  -node-label-resync-interval duration
    	Frequency of node label resync. (default 1h0m0s)
  -node-label-sync-workers int
    	Maximum concurrent node label sync operations. (default 5)
  -pvc-label-resync-delay duration
    	Startup delay of initial PVC label resync. (default 5s)
  -pvc-label-resync-interval duration
    	Frequency of PVC label resync. (default 1h0m0s)
  -pvc-label-sync-workers int
    	Maximum concurrent PVC label sync operations. (default 5)
  -zap-devel
    	Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)
  -zap-encoder value
    	Zap log encoding (one of 'json' or 'console')
  -zap-log-level value
    	Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  -zap-stacktrace-level value
    	Zap Level at and above which stacktraces are captured (one of 'info', 'error').
```

## Setup/Development

Ensure a k8s cluster is running and ~/.kube/config contains the kubeconfig, or
pass the path to kubeconfig to the manager binary with -kubeconfig flag.

```console
# Build the binary.
$ make manager

# Update the manifests (Optional).
$ make manifests

# Install the CRD.
$ make install

# Run the manager locally.
$ bin/manager -api-endpoint 172.17.0.2
2020-09-14T10:02:03.644+0100	DEBUG	setup	connected to the storageos api
2020-09-14T10:02:03.948+0100	INFO	controller-runtime.metrics	metrics server is starting to listen	{"addr": ":8080"}
2020-09-14T10:02:03.949+0100	INFO	controller-runtime.manager	starting metrics server	{"path": "/metrics"}
```
