package scheduler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/provisioner"
)

const (
	// PodSchedulerAnnotationKey is the pod annotation key that can be set to
	// skip pod scheduler name mutation.
	PodSchedulerAnnotationKey = "storageos.com/scheduler"
)

// PodSchedulerSetter is responsible for mutating and setting pod scheduler
// name.
type PodSchedulerSetter struct {
	// Provisioners is a list of storage provisioners to check a pod volume
	// against.
	Provisioners []string

	// SchedulerName is the name of the scheduler to mutate pods with.
	SchedulerName string

	// SchedulerAnnotationKey is the pod annotation that can be set to skip or
	// apply the mutation.
	SchedulerAnnotationKey string

	client.Client
	log logr.Logger
}

// NewPodSchedulerSetter returns a new Pod Scheduler mutating admission
// controller.
func NewPodSchedulerSetter(k8s client.Client, scheduler string) *PodSchedulerSetter {
	return &PodSchedulerSetter{
		SchedulerName:          scheduler,
		Provisioners:           []string{provisioner.DriverName},
		SchedulerAnnotationKey: PodSchedulerAnnotationKey,
		Client:                 k8s,
		log:                    ctrl.Log,
	}
}

// MutatePod mutates a given pod with a configured scheduler name if the pod is
// associated with volumes managed by the configured provisioners.
//
// Errors returned here may block creation of the Pod, depending on the Webhook
// configuration.
func (p *PodSchedulerSetter) MutatePod(ctx context.Context, pod *corev1.Pod, namespace string) error {
	log := p.log.WithValues("scheduler", p.SchedulerName, "pod", client.ObjectKeyFromObject(pod).String())
	log.V(4).Info("received pod for mutation")

	// Skip processing if no scheduler configured.
	if p.SchedulerName == "" {
		log.V(4).Info("pod scheduler not set, skipping")
		return nil
	}

	// Skip mutation if the pod annotation has false schedule annotation.
	if val, exists := pod.ObjectMeta.Annotations[p.SchedulerAnnotationKey]; exists {
		enabled, err := strconv.ParseBool(val)
		// No error in parsing and the value is false, skip the pod.
		if err == nil && !enabled {
			log.V(4).Info(fmt.Sprintf("pod has %s=false annotation, skipping", p.SchedulerAnnotationKey))
			return nil
		}
	}

	managedVols := []corev1.Volume{}

	// Find all the managed volumes.
	for _, vol := range pod.Spec.Volumes {
		ok, err := provisioner.IsStorageOSVolume(p.Client, vol, namespace)
		if err != nil {
			return errors.Wrap(err, "failed to determine if the volume is managed")
		}
		if ok {
			managedVols = append(managedVols, vol)
		}
	}

	// Set scheduler name only if there are managed volumes.
	if len(managedVols) == 0 {
		log.V(4).Info("pod does not have storageos volumes, skipping")
		return nil
	}

	pod.Spec.SchedulerName = p.SchedulerName
	log.Info("set scheduler")
	return nil
}
