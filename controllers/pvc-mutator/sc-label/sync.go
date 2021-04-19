package sclabel

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LabelSetter is responsible for setting labels
// based on given StorageClass's parameters.
type LabelSetter struct {
	client.Client
	log logr.Logger
}

// NewLabelSetter returns a new PVC label sync mutating admission
// controller.
func NewLabelSetter(k8s client.Client) *LabelSetter {
	return &LabelSetter{
		Client: k8s,
		log:    ctrl.Log,
	}
}

// MutatePVC mutates a given pvc with labels based on,
// StoraeClass params.
//
// Errors returned here may block creation of the PVC, depending on the
// FailurePolicy set in the webhook configuration.
func (s *LabelSetter) MutatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) error {
	log := s.log.WithName("StorageClassParamsLabelSetter").
		WithValues("pvc", client.ObjectKeyFromObject(pvc).String())
	log.V(4).Info("received pvc for mutation")

	// Skip mutation if the PVC is not provisioned by StorageOS
	provisioned, err := provisioner.IsProvisionedPVC(s.Client, *pvc, namespace, provisioner.DriverName)
	if err != nil {
		return errors.Wrap(err, "failed to check pvc provisioner")
	}
	if !provisioned {
		log.V(4).Info("pvc does not provisioned by StorageOS, skipping")
		return nil
	}

	// Find StorageClass of PVC
	storageClass, err := provisioner.StorageClassForPVC(s.Client, pvc)
	if err != nil {
		return err
	}

	// Set labels on the PVC based on parameters
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	for k, v := range storageClass.Parameters {
		if _, ok := pvc.Labels[k]; ok {
			continue
		}
		v := v
		pvc.Labels[k] = v
	}

	log.Info("set StorageClass parameters as labels")
	return nil
}
