package keys

import (
	"context"
	"encoding/hex"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/crypto"
)

var (
	// ErrNoKeyInSecret is returned when a secret was expected to contain an
	// encryption key, but didn't.
	ErrNoKeyInSecret = errors.New("secret does not contain encryption key")
)

// KeyManager generates, stores and removes encryption keys.
type KeyManager struct {
	client client.Client
}

// New creates a new KeyManager that is responsible for generationg and storing
// volume encryption keys.  The client should be uncached so that created
// secrets can be read back immediately.
func New(client client.Client) *KeyManager {
	return &KeyManager{client: client}
}

// Ensure that a secret exists at volKeyRef, creating it with valid keys if
// needed.  If a secret already exists, it does not verify validity.
func (m *KeyManager) Ensure(ctx context.Context, nsKeyRef client.ObjectKey, volKeyRef client.ObjectKey, nsSecretLabels map[string]string, volSecretLabels map[string]string) error {
	// Return immediately if the volume key already exists, or an error if
	// another error (e.g. permission denied).
	err := m.client.Get(ctx, volKeyRef, &corev1.Secret{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// We need to generate a new volume key.  Check if the namespace key exists,
	// and create first if needed.
	nsKey, err := m.ensureNamespaceKey(ctx, nsKeyRef, nsSecretLabels)
	if err != nil {
		return err
	}
	return m.createVolumeKey(ctx, volKeyRef, nsKey, volSecretLabels)
}

func (m *KeyManager) ensureNamespaceKey(ctx context.Context, nsKeyRef client.ObjectKey, labels map[string]string) (string, error) {
	existing := &corev1.Secret{}
	err := m.client.Get(ctx, nsKeyRef, existing)
	if err == nil {
		// Key exists, use it or error if invalid.
		return keyFromSecret(existing)
	}
	if !apierrors.IsNotFound(err) {
		return "", err
	}

	key, err := crypto.GenerateUserKey()
	if err != nil {
		return "", err
	}

	data := map[string][]byte{
		"key": key,
	}

	secret := m.secret(nsKeyRef, data, labels)
	if err := m.client.Create(ctx, secret); err != nil {
		return "", err
	}
	return keyFromSecret(secret)
}

// createVolumeKey generates a new volume key and stores it in a secret at
// volKeyRef.
//
// Will return an error if the secret already exists.
func (m *KeyManager) createVolumeKey(ctx context.Context, volKeyRef client.ObjectKey, nsKey string, labels map[string]string) error {
	// Generate Initialization Vector.
	iv, err := crypto.GenerateIV()
	if err != nil {
		return err
	}

	// Generate Volume Master Key.
	vmk, err := crypto.GenerateVMK()
	if err != nil {
		return err
	}

	nskeyBytes, err := hex.DecodeString(nsKey)
	if err != nil {
		return err
	}

	ik, err := crypto.CreateIK(nskeyBytes, iv)
	if err != nil {
		return err
	}

	// Encrypt the VMK with ik to obtain a Volume User Key.
	vuk, err := crypto.Encrypt(vmk, ik)
	if err != nil {
		return err
	}

	// Create a HMAC of the VMK.
	hmac, err := crypto.CreateHMAC(vmk, iv)
	if err != nil {
		return err
	}

	// Secret contents.
	data := map[string][]byte{
		"key":  vmk,
		"iv":   iv,
		"vuk":  vuk,
		"hmac": hmac,
	}

	return m.client.Create(ctx, m.secret(volKeyRef, data, labels))
}

// secret returns a secret object.  The owner is not set as we don't want keys
// to be deleted when the api-manager is upgraded.  In the future we can add
// finalizer-based garbage collection.  For now, there is no garbage collection.
func (m *KeyManager) secret(key client.ObjectKey, data map[string][]byte, labels map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

func keyFromSecret(secret *corev1.Secret) (string, error) {
	if secret.Data == nil {
		return "", ErrNoKeyInSecret
	}
	if key, ok := secret.Data["key"]; ok && len(key) > 0 {
		return hex.EncodeToString(key), nil
	}
	return "", ErrNoKeyInSecret
}
