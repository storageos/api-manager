package encryption

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/storageos/api-manager/controllers/pvc-mutator/encryption/keys"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// EncryptionEnabledLabel label must be set to true to enable encryption for
	// the pvc.
	EncryptionEnabledLabel = "storageos.com/encryption"

	// EncryptionSecretNameAnnotationKey is the name of the pvc annotation to
	// store the encryption secret name in.
	EncryptionSecretNameAnnotationKey = "storageos.com/encryption-secret-name"

	// EncryptionSecretNamespaceAnnotationKey is the name of the pvc annotation
	// to store the encryption secret namespace in.
	EncryptionSecretNamespaceAnnotationKey = "storageos.com/encryption-secret-namespace"

	// VolumeSecretNamePrefix will be used to prefix all volume key secrets.
	VolumeSecretNamePrefix = "storageos-volume-key"

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
	Ensure(ctx context.Context, userKeyRef client.ObjectKey, volKeyRef client.ObjectKey) error
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

	client.Client
	keys KeyManager
	log  logr.Logger
}

// NewEncryptionKeySetter returns a new PVC encryption key mutating admission
// controller.
func NewEncryptionKeySetter(k8s client.Client) *EncryptionKeySetter {
	return &EncryptionKeySetter{
		enabledLabel:                 EncryptionEnabledLabel,
		secretNameAnnotationKey:      EncryptionSecretNameAnnotationKey,
		secretNamespaceAnnotationKey: EncryptionSecretNamespaceAnnotationKey,

		Client: k8s,
		keys:   keys.New(k8s),
		log:    ctrl.Log,
	}
}

// MutatePVC mutates a given pvc with annotations containing its encryption key,
// if the pvc has encryption enabled.
//
// Errors returned here will block creation of the PVC.
func (s *EncryptionKeySetter) MutatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim, namespace string) error {
	log := s.log.WithValues("pvc", client.ObjectKeyFromObject(pvc).String())
	log.V(4).Info("received pvc for mutation")

	// Skip mutation if the PVC does not have encryption enabled.  Don't bother checking
	// the StorageClass to make sure it's StorageOS.  The encryption label
	// should only be added to StorageOS PVCs.
	if !isEnabled(s.enabledLabel, pvc.GetLabels()) {
		log.V(4).Info(fmt.Sprintf("pvc does not have %s=true annotation, skipping", s.enabledLabel))
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

	if err := s.keys.Ensure(ctx, nsKeyRef, volKeyRef); err != nil {
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

// isEnabled returns true if the key is set in the kv map and its value is true.
func isEnabled(key string, kv map[string]string) bool {
	val, exists := kv[key]
	if !exists {
		return false
	}

	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return boolVal
}
