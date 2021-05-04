package encryption

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/storageos/api-manager/controllers/pvc-mutator/encryption/keys"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SecretNameAnnotationKey is the name of the pvc annotation to store the
	// encryption secret name in.
	SecretNameAnnotationKey = "storageos.com/encryption-secret-name"

	// SecretNamespaceAnnotationKey is the name of the pvc annotation to store
	// the encryption secret namespace in.
	SecretNamespaceAnnotationKey = "storageos.com/encryption-secret-namespace"

	// VolumeSecretNamePrefix will be used to prefix all volume key secrets.
	VolumeSecretNamePrefix = "storageos-volume-key"

	// VolumeSecretPVCNameLabel is used to set the reference to the PVC name on
	// the volume key secret.  The namespace is not needed as it will be the
	// same as the secret.
	VolumeSecretPVCNameLabel = "storageos.com/pvc"

	// NamespaceSecretName is the name of the secret containing the user key in
	// each namespace with encrypted volumes.
	NamespaceSecretName = "storageos-namespace-key"
)

var (
	// ErrCrossNamespace is returned if a encryption key secret is requested
	// that is not it the PVC namespace.
	ErrCrossNamespace = errors.New("encryption key secret namespace must match pvc namespace")
)

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// KeyManager is the encrption key manager, responsible for creating and
// retrieving secrets that contain the keys required for volume encryption.
type KeyManager interface {
	Ensure(ctx context.Context, userKeyRef client.ObjectKey, volKeyRef client.ObjectKey, nsSecretLabels map[string]string, volSecretLabels map[string]string) error
}

// EncryptionKeySetter is responsible for generating and setting pvc encryption
// keys on a pvc.
type EncryptionKeySetter struct {
	// enabledLabel is the pvc label used to indicate that the volume
	// must have encryption enabled.  It must be set to "true" to enable.
	enabledLabel string

	// secretNameAnnotationKey is the pvc annotation that stores the name of the
	// secret containing the encryption key.
	secretNameAnnotationKey string

	// secretNamespaceAnnotationKey is the pvc annotation that stores the
	// namespace of the secret containing the encryption key.
	secretNamespaceAnnotationKey string

	// labels that should be applied to any kubernetes resources created by the
	// key manager.
	labels map[string]string

	client.Client
	keys KeyManager
	log  logr.Logger
}

// NewKeySetter returns a new PVC encryption key mutating admission
// controller that generates volume encryption keys and sets references to their
// location as PVC annotations.
func NewKeySetter(k8s client.Client, labels map[string]string) *EncryptionKeySetter {
	return &EncryptionKeySetter{
		enabledLabel:                 storageos.ReservedLabelEncryption,
		secretNameAnnotationKey:      SecretNameAnnotationKey,
		secretNamespaceAnnotationKey: SecretNamespaceAnnotationKey,

		Client: k8s,
		keys:   keys.New(k8s),
		labels: labels,
		log:    ctrl.Log.WithName("keygen"),
	}
}

// MutatePVC mutates a given pvc with annotations containing its encryption key,
// if the pvc has encryption enabled.
//
// Errors returned here may block creation of the PVC, depending on the
// FailurePolicy set in the webhook configuration.
func (s *EncryptionKeySetter) MutatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) error {
	log := s.log.WithValues("pvc", client.ObjectKeyFromObject(pvc).String())
	log.V(4).Info("received pvc for mutation")

	// Retrieve StorageClass of the PVC.
	storageClass, err := provisioner.StorageClassForPVC(s.Client, pvc)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve storageclass of pvc")
	}

	// Skip mutation if the PVC is not provisioned by StorageOS
	provisioned := provisioner.IsProvisionedStorageClass(storageClass, provisioner.DriverName)
	if !provisioned {
		log.V(4).Info("pvc does not provisioned by StorageOS, skipping")
		return nil
	}

	// Skip mutation if the PVC does not have encryption enabled.
	// The encryption label should be added to StorageOS PVCs
	// or inherited from StorageOS StorageClass.
	// Invalid value of encryption must block PVC creation.
	enabled, err := s.isEnabled(pvc.GetLabels(), storageClass.Parameters)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to parse boolean value of %q pvc", s.enabledLabel))
	}
	if !enabled {
		log.V(4).Info("pvc does not have encryption enabled, skipping")
		return nil
	}

	// Do not allow secrets from another namespace to be referenced.  We assume
	// that if the user can create PVCs in the namespace then they should be
	// able to read volume secrets from that namespace.  We can't assume the
	// same for other namespaces.
	if requested, ok := pvc.Annotations[s.secretNamespaceAnnotationKey]; ok && requested != namespace {
		return ErrCrossNamespace
	}

	// Ensure the keys exist, creating them if not.
	nsKeyRef := s.NamespaceSecretKeyRef(namespace)
	volKeyRef := s.VolumeSecretKeyRef(pvc, namespace)

	// Add the PVC reference to the volume key secret labels.  This is
	// convenience only and should not be relied on.  The volume key should
	// really relate to the PV, not the PVC but the PV hasn't been provisioned
	// yet.
	volSecretLabels := s.VolumeSecretLabels(pvc.GetName())

	// Ensure that the encryption keys exist where expected, creating them if
	// needed.
	if err := s.keys.Ensure(ctx, nsKeyRef, volKeyRef, s.labels, volSecretLabels); err != nil {
		return errors.Wrap(err, "failed to ensure encryption key present for pvc")
	}

	// Set annotations on the PVC pointing to the volume key secret.  The
	// namespace key secret does not need to be passed to the control plane.
	if pvc.Annotations == nil {
		pvc.Annotations = make(map[string]string)
	}
	pvc.Annotations[s.secretNameAnnotationKey] = volKeyRef.Name
	pvc.Annotations[s.secretNamespaceAnnotationKey] = volKeyRef.Namespace

	log.Info("set volume encryption key annotations")
	return nil
}

// VolumeSecretLabels returns the labels that should be set on the volume key
// secret.
func (s *EncryptionKeySetter) VolumeSecretLabels(pvcName string) map[string]string {
	labels := make(map[string]string)
	for k, v := range s.labels {
		labels[k] = v
	}

	// pvcName should never be empty, but no point setting if it is.
	if pvcName != "" {
		labels[VolumeSecretPVCNameLabel] = pvcName
	}

	return labels
}

// NamespaceSecretKeyRef returns the reference of the secret that should be used to
// store the user encryption key for a namespace.
//
// This key is used to create volume keys.
func (s *EncryptionKeySetter) NamespaceSecretKeyRef(pvcNamespace string) client.ObjectKey {
	return client.ObjectKey{
		Name:      NamespaceSecretName,
		Namespace: pvcNamespace,
	}
}

// VolumeSecretKeyRef returns the reference of the secret that should be used to
// store the encryption keys for a volume provisioned by the PVC.
func (s *EncryptionKeySetter) VolumeSecretKeyRef(pvc *corev1.PersistentVolumeClaim, pvcNamespace string) client.ObjectKey {
	annotations := pvc.GetAnnotations()
	name, ok := annotations[s.secretNameAnnotationKey]
	if !ok || name == "" {
		name = GenerateVolumeSecretName()
	}
	namespace, ok := annotations[s.secretNamespaceAnnotationKey]
	if !ok || namespace == "" {
		namespace = pvcNamespace
	}

	return client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
}

// GenerateVolumeSecretName returns the name of the secret to use for the volume
// key.
//
// The secret relates to the StorageOS volume (or Kubernetes PV), not the PVC
// which may be deleted and then the PV reused.  Since the volume hasn't been
// provisioned yet we don't have a reference for it, so generate a unique
// identifier to use instead.
func GenerateVolumeSecretName() string {
	return fmt.Sprintf("%s-%s", VolumeSecretNamePrefix, uuid.New().String())
}

// isEnabled iterates on the given maps and looks for encryption key. First occurrence wins
func (s *EncryptionKeySetter) isEnabled(hayStacks ...map[string]string) (bool, error) {
	for _, hayStack := range hayStacks {
		val, exists := hayStack[s.enabledLabel]
		if exists {
			return strconv.ParseBool(val)
		}
	}

	return false, nil
}
