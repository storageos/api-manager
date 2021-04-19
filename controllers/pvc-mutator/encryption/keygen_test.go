package encryption

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/storageos/api-manager/controllers/pvc-mutator/encryption/keys"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMutatePVC(t *testing.T) {
	t.Parallel()

	// Test values only.
	storageosProvisioner := provisioner.DriverName

	// Create a new scheme and add all the types from different clientsets.
	scheme := runtime.NewScheme()
	if err := kscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// StorageOS StorageClass.
	stosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "stos",
		},
		Provisioner: storageosProvisioner,
	}

	// Non-StorageOS StorageClass.
	notstosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-stos",
		},
		Provisioner: "foo-provisioner",
	}

	testNamespace := "default"

	testcases := []struct {
		name                              string
		namespace                         string
		notStos                           bool
		betaAnnotation                    bool
		labels                            map[string]string
		annotations                       map[string]string
		wantSecretNameAnnotationGenerated bool
		wantSecretNameAnnotation          string
		wantSecretNamespaceAnnotation     string
		wantErr                           bool
	}{
		{
			name:                          "pvc without encryption",
			namespace:                     testNamespace,
			wantSecretNameAnnotation:      "",
			wantSecretNamespaceAnnotation: "",
		},
		{
			name:      "pvc with encryption",
			namespace: testNamespace,
			labels: map[string]string{
				EnabledLabel: "true",
			},
			wantSecretNameAnnotationGenerated: true,
			wantSecretNamespaceAnnotation:     testNamespace,
		},
		{
			name:      "pvc with user-specifed secret name",
			namespace: testNamespace,
			labels: map[string]string{
				EnabledLabel: "true",
			},
			annotations: map[string]string{
				SecretNameAnnotationKey: "my-secret-name",
			},
			wantSecretNameAnnotation:      "my-secret-name",
			wantSecretNamespaceAnnotation: testNamespace,
		},
		{
			name:      "pvc with user specifed secret namespace",
			namespace: testNamespace,
			labels: map[string]string{
				EnabledLabel: "true",
			},
			annotations: map[string]string{
				SecretNamespaceAnnotationKey: testNamespace,
			},
			wantSecretNameAnnotationGenerated: true,
			wantSecretNamespaceAnnotation:     testNamespace,
		},
		{
			name:      "pvc with user specifed secret namespace to another namespace",
			namespace: testNamespace,
			labels: map[string]string{
				EnabledLabel: "true",
			},
			annotations: map[string]string{
				SecretNamespaceAnnotationKey: "another-users-ns",
			},
			wantErr: true,
		},
		{
			name:      "pvc with user-specifed secret name and namespace",
			namespace: testNamespace,
			labels: map[string]string{
				EnabledLabel: "true",
			},
			annotations: map[string]string{
				SecretNameAnnotationKey:      "my-secret-name",
				SecretNamespaceAnnotationKey: testNamespace,
			},
			wantSecretNameAnnotation:      "my-secret-name",
			wantSecretNamespaceAnnotation: testNamespace,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Create all the above resources and get a k8s client.
			k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stosSC, notstosSC).Build()

			// Create a EncryptionKeySetter instance with the fake client.
			encryptionKeySetter := EncryptionKeySetter{
				enabledLabel:                 EnabledLabel,
				secretNameAnnotationKey:      SecretNameAnnotationKey,
				secretNamespaceAnnotationKey: SecretNamespaceAnnotationKey,

				Client: k8s,
				keys:   keys.New(k8s),
				log:    ctrl.Log,
			}

			scName := stosSC.Name
			if tc.notStos {
				scName = notstosSC.Name
			}

			pvc := createPVC("pvc1", tc.namespace, scName, tc.betaAnnotation, tc.labels, tc.annotations)

			err := encryptionKeySetter.MutatePVC(context.Background(), pvc, tc.namespace)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("got unexpected error: %v", err)
				}
				return
			}

			// Check name ref annotation.
			annotations := pvc.GetAnnotations()
			nameRef := annotations[SecretNameAnnotationKey]
			if tc.wantSecretNameAnnotationGenerated && !strings.HasPrefix(nameRef, VolumeSecretNamePrefix) {
				t.Errorf("expected %s annotation to be generated, got %s", SecretNameAnnotationKey, nameRef)
			}
			if tc.wantSecretNameAnnotation != "" && nameRef != tc.wantSecretNameAnnotation {
				t.Errorf("expected %s annotation to be set to %s, got %s", SecretNameAnnotationKey, tc.wantSecretNameAnnotation, nameRef)
			}
			if !tc.wantSecretNameAnnotationGenerated && tc.wantSecretNameAnnotation == "" && nameRef != "" {
				t.Errorf("expected %s annotation to be unset, got %s", SecretNameAnnotationKey, nameRef)
			}

			// Check namespace ref annotation.
			nsRef := annotations[SecretNamespaceAnnotationKey]
			switch tc.wantSecretNamespaceAnnotation == "" {
			case true:
				if nsRef != "" {
					t.Errorf("expected %s annotation to be unset, got %s", SecretNamespaceAnnotationKey, nsRef)
				}
			case false:
				if nsRef != tc.wantSecretNamespaceAnnotation {
					t.Errorf("expected %s annotation to be set to %s, got %s", SecretNamespaceAnnotationKey, tc.wantSecretNamespaceAnnotation, nsRef)
				}
			}

			if tc.wantSecretNameAnnotation == "" && tc.wantSecretNamespaceAnnotation == "" {
				return
			}

			ctx := context.Background()

			// Check namespace secret exists.
			nsSecret := &corev1.Secret{}
			nsSecretRef := client.ObjectKey{
				Name:      NamespaceSecretName,
				Namespace: tc.namespace,
			}

			if err := k8s.Get(ctx, nsSecretRef, nsSecret); err != nil {
				t.Fatalf("failed to get namespace secret: %v", err)
			}

			v, ok := nsSecret.Data["key"]
			if !ok || len(v) != 32 {
				t.Errorf("expected key in namespace secret, got: %s", string(v))
			}

			// Check volume secret exists.
			volSecret := &corev1.Secret{}
			volSecretRef := client.ObjectKey{
				Name:      nameRef,
				Namespace: nsRef,
			}
			if err := k8s.Get(ctx, volSecretRef, volSecret); err != nil {
				t.Fatalf("failed to get volume secret: %v", err)
			}

			// Check secret contents.  Only "key" is required.
			v, ok = volSecret.Data["key"]
			if !ok || len(v) != 64 {
				t.Errorf("expected key in volume secret, got: %s", string(v))
			}
		})
	}
}

// createPVC creates and returns a PVC object.
func createPVC(name, namespace, storageClassName string, betaAnnotation bool, labels map[string]string, annotations map[string]string) *corev1.PersistentVolumeClaim {
	scAnnotationKey := "volume.beta.kubernetes.io/storage-class"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	// Don't set storage class name if using default storage class.
	if storageClassName == "" {
		return pvc
	}

	if betaAnnotation {
		if pvc.ObjectMeta.Annotations == nil {
			pvc.ObjectMeta.Annotations = make(map[string]string)
		}
		pvc.ObjectMeta.Annotations[scAnnotationKey] = storageClassName
	} else {
		pvc.Spec.StorageClassName = &storageClassName
	}

	return pvc
}

func Test_isEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		kv   map[string]string
		want bool
	}{
		{
			name: "enabled",
			key:  "foo",
			kv: map[string]string{
				"foo": "true",
			},
			want: true,
		},
		{
			name: "disabled",
			key:  "foo",
			kv: map[string]string{
				"foo": "false",
			},
			want: false,
		},
		{
			name: "empty value",
			key:  "foo",
			kv: map[string]string{
				"foo": "",
			},
			want: false,
		},
		{
			name: "empty map",
			key:  "foo",
			kv:   map[string]string{},
			want: false,
		},
		{
			name: "nil map",
			key:  "foo",
			kv:   nil,
			want: false,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			if got := isEnabled(tt.key, tt.kv); got != tt.want {
				t.Errorf("isEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncryptionKeySetter_VolumeSecretLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pvcName string
		labels  map[string]string
		want    map[string]string
	}{
		{
			name:    "default",
			pvcName: "pvc1",
			labels: map[string]string{
				"foo": "bar",
			},
			want: map[string]string{
				VolumeSecretPVCNameLabel: "pvc1",
				"foo":                    "bar",
			},
		},
		{
			name:    "empty pvc name",
			pvcName: "",
			labels: map[string]string{
				"foo": "bar",
			},
			want: map[string]string{
				"foo": "bar",
			},
		},
		{
			name:    "empty base labels",
			pvcName: "pvc1",
			labels:  map[string]string{},
			want: map[string]string{
				VolumeSecretPVCNameLabel: "pvc1",
			},
		},
		{
			name:    "nil base labels",
			pvcName: "pvc1",
			labels:  nil,
			want: map[string]string{
				VolumeSecretPVCNameLabel: "pvc1",
			},
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			s := &EncryptionKeySetter{
				enabledLabel:                 EnabledLabel,
				secretNameAnnotationKey:      SecretNameAnnotationKey,
				secretNamespaceAnnotationKey: SecretNamespaceAnnotationKey,
				labels:                       tt.labels,
				Client:                       nil,
				keys:                         keys.New(nil),
				log:                          ctrl.Log,
			}
			if got := s.VolumeSecretLabels(tt.pvcName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EncryptionKeySetter.VolumeSecretLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
