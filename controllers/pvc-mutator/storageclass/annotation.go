package storageclass

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AnnotationSetter is responsible to add an annotation
// to link the StorageClass with the PVC.
type AnnotationSetter struct {
	client.Client
	log logr.Logger
}

// NewAnnotationSetter returns a new PVC annotation mutating admission
// controller.
func NewAnnotationSetter(k8s client.Client) *AnnotationSetter {
	return &AnnotationSetter{
		Client: k8s,
		log:    ctrl.Log.WithName("storageclass"),
	}
}

// MutatePVC mutates a given pvc with a new annotation,
// by attached StorageClass.
//
// Errors returned here may block creation of the PVC, depending on the
// FailurePolicy set in the webhook configuration.
func (s *AnnotationSetter) MutatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) error {
	log := s.log.WithValues("pvc", client.ObjectKeyFromObject(pvc).String())
	log.V(4).Info("received pvc for mutation")

	// Find StorageClass of PVC.
	storageClass, err := provisioner.StorageClassForPVC(s.Client, pvc)
	if err != nil {
		return errors.Wrap(err, "failed to check pvc provisioner")
	}

	// Skip mutation if the PVC will not be provisioned by StorageOS.
	if provisioned := provisioner.IsProvisionedStorageClass(storageClass, provisioner.DriverName); !provisioned {
		log.V(4).Info("pvc will not be provisioned by StorageOS, skipping")
		return nil
	}

	// Set annotation on the PVC based on StorageClass.
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations[provisioner.StorageClassUUIDAnnotationKey] = string(storageClass.UID)

	log.Info("set StorageClass UID as annotation", "pvc", pvc.Name, "uid", string(storageClass.UID))
	return nil
}
