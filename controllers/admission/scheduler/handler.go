package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
	decoder *admission.Decoder
	log     logr.Logger
}

// Check if the Handler interface is implemented.
var _ admission.Handler = &PodSchedulerSetter{}

// +kubebuilder:webhook:path=/mutate-pods,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=pod-mutator.storageos.com,admissionReviewVersions=v1

// NewPodSchedulerSetter returns a new Pod Scheduler mutating admission
// controller.
func NewPodSchedulerSetter(k8s client.Client, decoder *admission.Decoder, scheduler string) *PodSchedulerSetter {
	return &PodSchedulerSetter{
		SchedulerName:          scheduler,
		Provisioners:           []string{provisioner.DriverName},
		SchedulerAnnotationKey: PodSchedulerAnnotationKey,
		Client:                 k8s,
		decoder:                decoder,
		log:                    ctrl.Log,
	}
}

// Handle handles an admission request and mutates a pod object in the request.
func (p *PodSchedulerSetter) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := p.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Get the request namespace. This is needed because sometimes the decoded
	// pod object lacks namespace info.
	namespace := req.AdmissionRequest.Namespace

	// Create a copy of the pod to mutate.
	copy := pod.DeepCopy()
	if err := p.mutatePodsFn(ctx, copy, namespace); err != nil {
		p.log.Error(err, "failed to set pod scheduler")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(copy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// mutatePodFn mutates a given pod with a configured scheduler name if the pod
// is associated with volumes managed by the configured provisioners.
func (p *PodSchedulerSetter) mutatePodsFn(ctx context.Context, pod *corev1.Pod, namespace string) error {
	log := p.log.WithValues("scheduler", p.SchedulerName, "pod", client.ObjectKeyFromObject(pod).String())
	log.V(4).Info("received pod for mutation")

	// Skip processing if no scheduler configured.
	if p.SchedulerName == "" {
		log.V(4).Info("pod scheduler not set, skipping")
		return nil
	}

	// Skip mutation if the pod annotation has false schedule annotation.
	if val, exists := pod.ObjectMeta.Annotations[p.SchedulerAnnotationKey]; exists {
		boolVal, err := strconv.ParseBool(val)
		// No error in parsing and the value is false, skip the pod.
		if err == nil && !boolVal {
			log.V(4).Info("pod has storageos.com/scheduler=false annotaion, skipping")
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
