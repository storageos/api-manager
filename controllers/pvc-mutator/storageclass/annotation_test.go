package storageclass

import (
	"context"
	"testing"

	"github.com/storageos/api-manager/internal/pkg/provisioner"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMutatePVCErrorToFetchStorageClass(t *testing.T) {
	// Create a new scheme and add all the types from different clientsets.
	scheme := runtime.NewScheme()
	if err := kscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// Create a fake client to fail on get StorageClass
	k8s := fake.NewClientBuilder().Build()

	// Create a AnnotationSetter instance with the fake client.
	annotationSetter := AnnotationSetter{
		Client: k8s,
		log:    ctrl.Log,
	}

	namespace := "namespace"

	// Create a pvc
	pvc := createPVC("pvc1", namespace, nil)

	err := annotationSetter.MutatePVC(context.Background(), pvc, namespace)
	if err == nil {
		t.Fatal("this must fail")
	}
}

func TestMutatePVC(t *testing.T) {
	// Create a new scheme and add all the types from different clientsets.
	scheme := runtime.NewScheme()
	if err := kscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// Default StorageOS StorageClass.
	defStosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("storageos-default-uid"),
			Name: "stos-default",
		},
		Provisioner: provisioner.DriverName,
	}

	// StorageOS StorageClass.
	stosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("storageos-uid"),
			Name: "stos",
		},
		Provisioner: provisioner.DriverName,
	}

	// Non-StorageOS StorageClass.
	notStosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("foo-uid"),
			Name: "non-stos",
		},
		Provisioner: "foo-provisioner",
	}

	testNamespace := "default"

	testcases := []struct {
		name         string
		namespace    string
		storageClass *storagev1.StorageClass
	}{
		{
			name:         "foreign StorageClass",
			namespace:    testNamespace,
			storageClass: notStosSC,
		},
		{
			name:         "default StorageClass",
			namespace:    testNamespace,
			storageClass: defStosSC,
		},
		{
			name:         "given StorageClass",
			namespace:    testNamespace,
			storageClass: stosSC,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Create all the above resources and get a k8s client.
			k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(defStosSC, stosSC, notStosSC).Build()

			// Create a AnnotationSetter instance with the fake client.
			annotationSetter := AnnotationSetter{
				Client: k8s,
				log:    ctrl.Log,
			}

			// Create a pvc
			pvc := createPVC("pvc1", tc.namespace, tc.storageClass)

			err := annotationSetter.MutatePVC(context.Background(), pvc, tc.namespace)
			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}

			// Collect annotation to test
			scAnnotation, ok := pvc.Annotations[provisioner.StorageClassUUIDAnnotationKey]

			// Validate result
			switch tc.storageClass.Provisioner {
			case provisioner.DriverName:
				if scAnnotation != string(tc.storageClass.UID) {
					t.Errorf("annotation value got:\n%s\n, want:\n%s", scAnnotation, string(tc.storageClass.UID))
				}
			default:
				if ok {
					t.Error("annotation found for foreign")
				}
			}
		})
	}
}

// createPVC creates and returns a PVC object.
func createPVC(name, namespace string, storageClass *storagev1.StorageClass) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

	if storageClass != nil {
		pvc.Spec.StorageClassName = &storageClass.Name
	}

	return pvc
}
