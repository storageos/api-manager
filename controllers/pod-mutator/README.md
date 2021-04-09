# Pod Mutator Admission Controller

The Pod Mutator is a mutating admission controller that modifies Pods during the
create process.

## Mutators

The Pod Mutator can run multiple mutation functions, each performing a different
task:

- The Pod Scheduler mutator adds the name of the StorageOS scheduler extender to
  the Pod's `SchedulerName`.  See [Pod Scheduler Mutator](controllers/pod-mutator/scheduler/README.md) for more detail.
  
## Tunables

Default values work well when the api-manager is installed by the
cluster-operator.  They should only be changed under advisement from StorageOS
support.

`-scheduler-name` is the name of the scheduler extender to set in the Pod's
`SchedulerName` field.  If set to an empty string, no updates will be made. When
set, the scheduler name must match the name of the scheduler extender configured
in the StorageOS cluster-operator. (default "storageos-scheduler)".

`-webhook-mutate-pods-path` is the URL path of the Pod mutating webhook. It
must match the configuration name set in the cluster-operator. (default
"/mutate-pods").
