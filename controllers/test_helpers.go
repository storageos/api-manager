package controllers

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
)

const (
	webhookMutatingConfigName = "mutating-webhook-configuration"
	webhookServiceName        = "webhook-service"
	webhookServiceNamespace   = "kube-system"
	webhookSecretName         = "storageos-webhook-secret"
	webhookSecretNamespace    = "kube-system"
	webhookMutatePodsPath     = "/mutate-pods"
	webhookMutatePVCsPath     = "/mutate-pvcs"
)

// testMutator is a test mutator that adds an annotation to pods or pvcs.
type testMutator struct {
	key   string
	value string
	error bool
}

func (m testMutator) MutatePod(ctx context.Context, obj *corev1.Pod, namespace string) error {
	if m.error {
		return errors.New("error")
	}
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[m.key] = m.value
	return nil
}

func (m testMutator) MutatePVC(ctx context.Context, obj *corev1.PersistentVolumeClaim, namespace string) error {
	if m.error {
		return errors.New("error")
	}
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[m.key] = m.value
	return nil
}
