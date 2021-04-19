package storageclasslabelsync

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StorageClassParamsLabelSetter is responsible for setting labels
// based on given StorageClass's params.
type StorageClassParamsLabelSetter struct {
	client.Client
	log logr.Logger
}

// NewStorageClassParamsLabelSetter returns a new PVC label sync mutating admission
// controller.
func NewStorageClassParamsLabelSetter(k8s client.Client) *StorageClassParamsLabelSetter {
	return &StorageClassParamsLabelSetter{
		Client: k8s,
		log:    ctrl.Log,
	}
}

// MutatePVC mutates a given pvc with labels based on,
// StoraeClass params.
//
// Errors returned here will block creation of the PVC.
func (s *StorageClassParamsLabelSetter) MutatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) error {
	log := s.log.WithValues("pvc", client.ObjectKeyFromObject(pvc).String())
	log.V(4).Info("received pvc for mutation")

	// Find StorageClass of PVC
	storageClass, err := provisioner.StorageClassForPVC(s.Client, pvc)
	if err != nil {
		log.V(4).Info("pvc does not have StorageClass, skipping")
		return err
	}

	// Set labels on the PVC based on params
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	for k, v := range storageClass.Parameters {
		v := v
		pvc.Labels[k] = v
	}

	log.Info("set StorageClass config as labels")
	return nil
}
