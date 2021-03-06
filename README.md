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

### Fencing Controller

The fencing controller performs fast failovers of Pods with StorageOS volumes
when a node is detected offline.

See [Fencing Contoller](controllers/fencer/README.md) for more detail.

### Node Delete Controller

The Node Delete Controller is responsible for removing nodes from the StorageOS
cluster when the node has been removed from Kubernetes.

See [Node Delete Contoller](controllers/node-delete/README.md) for more detail.

### Namespace Delete Controller

The Namespace Delete Controller is responsible for removing namespaces from the
StorageOS cluster when the namespace has been removed from Kubernetes.

See [Namespace Delete Controller](controllers/namespace-delete/README.md) for
more detail.

## Admission Controllers

Admission controllers intercept requests to the Kubernetes API prior to the
object being persisted, but after authentication and authorisation.

Mutating admission controllers run first and can modify the object, followed by
Validating admission controllers.  Both can cause the Kubernetes API to reject
the request.

### Pod Mutator Admission Controller

The Pod Mutator is a mutating admission controller that modifies Pods during the
create process.

See [Pod Mutator Admission Controller](controllers/pod-mutator/README.md) for
more detail.

### PVC Mutator Admission Controller

The PVC Mutator is a mutating admission controller that modifies
PersistentVolumeClaims during the create process.

See [PVC Mutator Admission Controller](controllers/pvc-mutator/README.md) for
more detail.

## Webhook Server

The admission controllers run as webhooks within api-manager. The webhook server
uses a self-signed certificate.

Certificates are rotated automatically and stored in a secret.  Multiple
instances of the api-manager share the same certificate and will check for
updates periodically, configured with the `-webhook-cert-refresh-interval` flag.

Certificates are valid for 1 year and re-issued after 6 months.  The
`-webhook-cert-refresh-interval` should be kept to run frequently (default
`30m`) as restarting the api-manager will reset the refresh timer.

It is not possible to disable the Webhook server or the admission controllers.
Instead, disable the individual mutation functions for each admission controller
so it becomes a no-op.

### Webhook server tunables

`-webhook-cert-refresh-interval` determines how often the webhook server
certificate should be checked for updates. (default 30m0s).

`-webhook-config-mutating` is the name of the mutating webhook configuration. It
must match the configuration name set in the cluster-operator. (default
"storageos-mutating-webhook").

`-webhook-secret-name` Is the name of the webhook secret. (default
"storageos-webhook").

`-webhook-secret-namespace` Is the namespace of the webhook secret.  If unset
(recommended), it will be auto-detected and set to the namespace that
api-manager is installed into.  If auto-detection is not available, it will
default to the value of `-namespace`, if set.

`-webhook-service-name` is the name of the webhook service. It must match the
configuration name set in the cluster-operator. (default "storageos-webhook").

`-webhook-service-namespace` is the namespace of the webhook service.  If unset
(recommended), it will be auto-detected and set to the namespace that
api-manager is installed into.  If auto-detection is not available, it will
default to the value of `-namespace`, if set.

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
  -api-refresh-interval duration
    	Frequency of StorageOS api authentication token refresh. (default 1m0s)
  -api-retry-interval duration
    	Frequency of StorageOS api retries on failure. (default 5s)
  -api-secret-path string
    	Path where the StorageOS api secret is mounted.  The secret must have "username" and "password" set. (default "/etc/storageos/secrets/api")
  -enable-leader-election
    	Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  -enable-node-label-sync
    	Enable node label sync controller. (default true)
  -enable-pvc-label-sync
    	Enable pvc label sync controller. (default true)
  -k8s-create-poll-interval duration
    	Frequency of Kubernetes api polling for new objects to appear once created. (default 1s)
  -k8s-create-wait-duration duration
    	Maximum time to wait for new Kubernetes objects to appear. (default 20s)
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -metrics-addr string
    	The address the metric endpoint binds to. (default ":8080")
  -namespace string
    	Namespace that the StorageOS components, including api-manager, are installed into.  Will be auto-detected if unset.
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
  -node-expiry-interval duration
    	Frequency of cached StorageOS node re-validation. (default 1h0m0s)
  -node-fencer-retry-interval duration
    	Frequency of fencing retries on failure. (default 5s)
  -node-fencer-timeout duration
    	Maximum time to wait for fencing to complete. (default 25s)
  -node-fencer-workers int
    	Maximum concurrent node fencing operations. (default 5)
  -node-label-resync-delay duration
    	Startup delay of initial node label resync. (default 10s)
  -node-label-resync-interval duration
    	Frequency of node label resync. (default 1h0m0s)
  -node-label-sync-workers int
    	Maximum concurrent node label sync operations. (default 5)
  -node-poll-interval duration
    	Frequency of StorageOS node polling. (default 5s)
  -pvc-label-resync-delay duration
    	Startup delay of initial PVC label resync. (default 5s)
  -pvc-label-resync-interval duration
    	Frequency of PVC label resync. (default 1h0m0s)
  -pvc-label-sync-workers int
    	Maximum concurrent PVC label sync operations. (default 5)
  -scheduler-name string
    	Name of the Pod scheduler to use for Pods with StorageOS volumes.  Set to an empty value to disable setting the Pod scheduler. (default "storageos-scheduler")
  -volume-expiry-interval duration
    	Frequency of cached StorageOS volume re-validation. (default 1m0s)
  -volume-poll-interval duration
    	Frequency of StorageOS volume polling. (default 5s)
  -webhook-cert-refresh-interval duration
    	Frequency of webhook certificate refresh. (default 30m0s)
  -webhook-cert-validity duration
    	Validity of webhook certificate. (default 8760h0m0s)
  -webhook-config-mutating string
    	Name of the mutating webhook configuration. (default "storageos-mutating-webhook")
  -webhook-mutate-pods-path string
    	URL path of the Pod mutating webhook. (default "/mutate-pods")
  -webhook-mutate-pvcs-path string
    	URL path of the PVC mutating webhook. (default "/mutate-pvcs")
  -webhook-secret-name string
    	Name of the webhook secret containing the certificate. (default "storageos-webhook")
  -webhook-secret-namespace string
    	Namespace of the webhook secret.  Will be auto-detected or value of -namespace if unset.
  -webhook-service-name string
    	Name of the webhook service. (default "storageos-webhook")
  -webhook-service-namespace string
    	Namespace of the webhook service.  Will be auto-detected or value of -namespace if unset.
  -zap-devel
    	Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)
  -zap-encoder value
    	Zap log encoding (one of 'json' or 'console')
  -zap-log-level value
    	Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  -zap-stacktrace-level value
    	Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
```

## Tracing

Opentelemetry tracing can be configured with a Jaeger exporter.  To enable, set
the following environment variables:

```console
JAEGER_DISABLED=false
JAEGER_ENDPOINT=http://<service-address>:14268/api/traces
```

This can be done by editing `deployment.apps/storageos-api-manager`:

```yaml
        env:
        - name: JAEGER_DISABLED
          value: "false"
        - name: JAEGER_ENDPOINT
          value: http://172.17.0.1:14268/api/traces
```

Tracing is not enabled by default.

To run a Jaeger endpoint, see:
https://www.jaegertracing.io/docs/1.21/getting-started/ 

Or for development purposes:

```console
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:1.21
```

The Web UI will be available at http://localhost:16686.

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
