# Pod Scheduler Admission Controller

This is a mutating admission controller that adds the name of the
StorageOS scheduler extender to the Pod's `SchedulerName`, if the Pod has
volumes backed by StorageOS and does not have the
`storageos.com/scheduler=false` annotation set on the Pod.

## Trigger

All Pod create events are evaluated.

Pods that have the annotation `storageos.com/scheduler=false` set are ignored.

The Pod's volumes are enumerated, and if at least one volume is backed by
StorageOS, the SchedulerName will be changed to use the StorageOS Pod scheduler.

## Disabling

Pod's can be skipped individually by setting the `storageos.com/scheduler=false`
annotation.

To disable for all Pods, the api-manager can be started with `-scheduler-name`
set to an empty string.

## Webhook server

The Pod Scheduler admission controller runs as a webhook within api-manager.
The webhook server is shared by other admission controllers and uses a
self-signed certificate.

Certificates are rotated automatically and stored in a secret.  Multiple
instances of the api-manager shared the same certificate and will check for
updates periodically, configured with the `-webhook-cert-refresh-interval` flag.

Certificates are valid for 1 year and re-issued after 6 months.  The
`-webhook-cert-refresh-interval` should be kept to run frequently (default
`30m`) as restarting the api-manager will reset the refresh timer.

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
