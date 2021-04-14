package podmutator

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/logr"
)

type Controller struct {
	client.Client
	mutators []Mutator
	decoder  *admission.Decoder
	log      logr.Logger
}

type Mutator interface {
	MutatePod(ctx context.Context, pod *corev1.Pod, namespace string) error
}

// Check if the Handler interface is implemented.
var _ admission.Handler = &Controller{}

// +kubebuilder:webhook:path=/mutate-pods,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=pod-mutator.storageos.com,admissionReviewVersions=v1

// NewController returns a new Pod mutating admission controller.
func NewController(k8s client.Client, decoder *admission.Decoder, mutators []Mutator) *Controller {
	return &Controller{
		mutators: mutators,
		Client:   k8s,
		decoder:  decoder,
		log:      ctrl.Log,
	}
}

// Handle handles an admission request and mutates a pod object in the request.
func (c *Controller) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := c.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Get the request namespace. This is needed because sometimes the decoded
	// pod object lacks namespace info.
	namespace := req.AdmissionRequest.Namespace

	// Run the mutators on the Pod object.
	for _, m := range c.mutators {
		if err := m.MutatePod(ctx, pod, namespace); err != nil {
			c.log.Error(err, "failed to mutate pod")
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
