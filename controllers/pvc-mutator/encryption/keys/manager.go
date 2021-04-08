package keys

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/crypto"
)

var (
	// ErrNoKeyInSecret is returned is a secret was expected to contain an
	// encryption key, but didn't.
	ErrNoKeyInSecret = errors.New("secret does not contain encryption key")
)

// KeyManager generates, stores and removes encryption keys.
type KeyManager struct {
	client client.Client
}

// New - creates a new DefaultManager.
func New(client client.Client) *KeyManager {
	return &KeyManager{client: client}
}

// Ensure that a secret exists at volKeyRef, creating it with valid keys if
// needed.  If a secret already exists, it does not verify validity.
func (m *KeyManager) Ensure(ctx context.Context, nsKeyRef client.ObjectKey, volKeyRef client.ObjectKey) error {
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
	nsKey, err := m.ensureNamespaceKey(ctx, nsKeyRef)
	if err != nil {
		return err
	}
	return m.createVolumeKey(ctx, volKeyRef, nsKey)
}

func (m *KeyManager) ensureNamespaceKey(ctx context.Context, nsKeyRef client.ObjectKey) (string, error) {
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

	secret := m.secret(nsKeyRef, data)
	if err := m.client.Create(ctx, secret); err != nil {
		return "", err
	}

	// Wait for secret to be created.  There's not much value in making these
	// configurable.  The create should be almost instant.
	ticker := time.NewTicker(50 * time.Millisecond)
	timeout := time.After(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err = m.client.Get(ctx, nsKeyRef, secret); err != nil {
				continue
			}
			return keyFromSecret(secret)
		case <-timeout:
			// Return last error.
			return "", err
		}
	}
}

// createVolumeKey generates a new volume key and stores it in a secret at
// volKeyRef.
//
// Will return an error if the secret already exists.
func (m *KeyManager) createVolumeKey(ctx context.Context, volKeyRef client.ObjectKey, nsKey string) error {
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

	return m.client.Create(ctx, m.secret(volKeyRef, data))
}

// TODO(croomes): should we add metadata here, i.e. reference to the namespace,
// `app.kubernetes.io/managed-by` or finalizer?
func (m *KeyManager) secret(key client.ObjectKey, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
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
