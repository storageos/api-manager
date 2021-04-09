package pvcmutator

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Controller struct {
	client.Client
	mutators []Mutator
	decoder  *admission.Decoder
	log      logr.Logger
}

type Mutator interface {
	MutatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) error
}

// Check if the Handler interface is implemented.
var _ admission.Handler = &Controller{}

// +kubebuilder:webhook:path=/mutate-pvcs,mutating=true,failurePolicy=ignore,sideEffects=None,groups="",resources=persistentvolumeclaims,verbs=create,versions=v1,name=pvc-mutator.storageos.com,admissionReviewVersions=v1

// NewController returns a new PVC mutating admission controller.
func NewController(k8s client.Client, decoder *admission.Decoder, mutators []Mutator) *Controller {
	return &Controller{
		mutators: mutators,
		Client:   k8s,
		decoder:  decoder,
		log:      ctrl.Log,
	}
}

// Handle handles an admission request and mutates a pvc object in the request.
func (c *Controller) Handle(ctx context.Context, req admission.Request) admission.Response {
	pvc := &corev1.PersistentVolumeClaim{}

	err := c.decoder.Decode(req, pvc)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Get the request namespace. This is needed because sometimes the decoded
	// pvc object lacks namespace info.
	namespace := req.AdmissionRequest.Namespace

	// Create a copy of the pod to mutate.
	copy := pvc.DeepCopy()
	for _, m := range c.mutators {
		if err := m.MutatePVC(ctx, copy, namespace); err != nil {
			c.log.Error(err, "failed to mutate pvc")
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	marshaledPVC, err := json.Marshal(copy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPVC)
}
