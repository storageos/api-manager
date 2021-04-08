# Pod Scheduler Mutator

The Pod Scheduler Mutator adds the name of the StorageOS scheduler extender to
the Pod's `SchedulerName`, if the Pod has volumes backed by StorageOS and does
not have the `storageos.com/scheduler=false` annotation set on the Pod.

## Trigger

All Pod create events are evaluated.

Pods that have the annotation `storageos.com/scheduler=false` set are ignored.

The Pod's volumes are enumerated, and if at least one volume is backed by
StorageOS, the `SchedulerName` will be changed to use the StorageOS Pod
scheduler.

## Disabling

Pod's can be skipped individually by setting the `storageos.com/scheduler=false`
annotation.

To disable for all Pods, the api-manager can be started with `-scheduler-name`
set to an empty string.

## Tunables

Default values work well when the api-manager is installed by the
cluster-operator.  They should only be changed under advisement from StorageOS
support.

`-scheduler-name` is the name of the scheduler extender to set in the Pod's
`SchedulerName` field.  If set to an empty string, no updates will be made. When
set, the scheduler name must match the name of the scheduler extender configured
in the StorageOS cluster-operator. (default "storageos-scheduler").
