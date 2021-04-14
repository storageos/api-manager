# Pod Mutator Admission Controller

The Pod Mutator is a mutating admission controller that modifies Pods during the
create process.

## Mutators

The Pod Mutator can run multiple mutation functions, each performing a different
task:

- The Pod Scheduler mutator adds the name of the StorageOS scheduler extender to
  the Pod's `SchedulerName`.  See [Pod Scheduler
  Mutator](controllers/pod-mutator/scheduler/README.md) for more detail.

## Webhook server

The admission controller runs as a webhook within api-manager.
The webhook server uses a self-signed certificate.

Certificates are rotated automatically and stored in a secret.  Multiple
instances of the api-manager share the same certificate and will check for
updates periodically, configured with the `-webhook-cert-refresh-interval` flag.

Certificates are valid for 1 year and re-issued after 6 months.  The
`-webhook-cert-refresh-interval` should be kept to run frequently (default
`30m`) as restarting the api-manager will reset the refresh timer.

## Disabling

It is not possible to disable the Webhook server.  Instead, disable the
individual mutation functions so the Pod Mutation becomes a no-op.

## Tunables

Default values work well when the api-manager is installed by the
cluster-operator.  They should only be changed under advisement from StorageOS
support.

`-scheduler-name` is the name of the scheduler extender to set in the Pod's
`SchedulerName` field.  If set to an empty string, no updates will be made. When
set, the scheduler name must match the name of the scheduler extender configured
in the StorageOS cluster-operator. (default "storageos-scheduler)".

`-webhook-cert-refresh-interval` determines how often the webhook server
certificate should be checked for updates. (default 30m0s).

`-webhook-config-mutating` is the name of the mutating webhook configuration. It
must match the configuration name set in the cluster-operator. (default
"storageos-mutating-webhook").

`-webhook-mutate-pods-path` is the URL path of the Pod mutating webhook. It
must match the configuration name set in the cluster-operator. (default
"/mutate-pods").

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