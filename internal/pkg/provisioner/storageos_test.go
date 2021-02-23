package provisioner

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_IsStorageOSNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		obj     client.Object
		want    bool
		wantErr bool
	}{
		{
			name: "no annotations",
			obj:  &corev1.Node{},
			want: false,
		},
		{
			name: "no csi annotations",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
		{
			name: "no storageos csi annotation",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":                   "bar",
						NodeDriverAnnotationKey: "{\"csi.xyz.com\":\"f4bfe4d3-fed0-47f0-bc89-e983670f25a9\"}",
					},
				},
			},
			want: false,
		},
		{
			name: "storageos csi annotation",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":                   "bar",
						NodeDriverAnnotationKey: "{\"csi.storageos.com\":\"f4bfe4d3-fed0-47f0-bc89-e983670f25a9\"}",
					},
				},
			},
			want: true,
		},
		{
			name: "badly formatted csi annotation",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":                   "bar",
						NodeDriverAnnotationKey: "{\"csi.storageos.com\":}",
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "not a node",
			obj:  &corev1.PersistentVolume{},
			want: false,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := IsStorageOSNode(tt.obj)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("NodeHasStorageOS() error = %v, wantErr %t", gotErr, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NodeHasStorageOS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsStorageOSPVC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  client.Object
		want bool
	}{
		{
			name: "no annotations",
			obj:  &corev1.PersistentVolumeClaim{},
			want: false,
		},
		{
			name: "no provisioner annotation",
			obj: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
		{
			name: "no storageos driver annotation",
			obj: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":                       "bar",
						PVCProvisionerAnnotationKey: "another-driver",
					},
				},
			},
			want: false,
		},
		{
			name: "storageos driver annotation",
			obj: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":                       "bar",
						PVCProvisionerAnnotationKey: DriverName,
					},
				},
			},
			want: true,
		},
		{
			name: "not a pvc",
			obj:  &corev1.Node{},
			want: false,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsStorageOSPVC(tt.obj); got != tt.want {
				t.Errorf("IsStorageOSPVC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsProvisionedVolume(t *testing.T) {
	t.Parallel()

	// Create a new scheme and add all the types from different clientsets.
	scheme := runtime.NewScheme()
	if err := kscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// StorageOS StorageClass.
	stosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fast",
		},
		Provisioner: DriverName,
	}

	// Non-StorageOS StorageClass.
	fooSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slow",
		},
		Provisioner: "foo-provisioner",
	}

	// StorageOS StorageClass set as default.
	stosSCdefault := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fast",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner: DriverName,
	}

	// Non-StorageOS StorageClass.
	fooSCdefault := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slow",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner: "foo-provisioner",
	}

	testNamespace := "default"

	tests := []struct {
		name           string
		storageClasses []*storagev1.StorageClass
		pvc            corev1.PersistentVolumeClaim
		volume         corev1.Volume
		want           bool
		wantErr        bool
	}{
		{
			name:           "storageos volume",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, stosSC.Name, false),
			volume:         createVolume("pv1"),
			want:           true,
		},
		{
			name:           "storageos volume, beta annotation",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, stosSC.Name, true),
			volume:         createVolume("pv1"),
			want:           true,
		},
		{
			name:           "storageos volume, default storage class",
			storageClasses: []*storagev1.StorageClass{stosSCdefault, fooSC},
			pvc:            createPVC("pv1", testNamespace, "", false),
			volume:         createVolume("pv1"),
			want:           true,
		},
		{
			name:           "non-storageos volume",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, fooSC.Name, false),
			volume:         createVolume("pv1"),
			want:           false,
		},
		{
			name:           "non-storageos volume, beta annotation",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, fooSC.Name, true),
			volume:         createVolume("pv1"),
			want:           false,
		},
		{
			name:           "non-storageos volume, default storage class",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSCdefault},
			pvc:            createPVC("pv1", testNamespace, "", true),
			volume:         createVolume("pv1"),
			want:           false,
		},
		{
			name:           "non-storageos volume, no storage classes",
			storageClasses: []*storagev1.StorageClass{},
			pvc:            createPVC("pv1", testNamespace, "", true),
			volume:         createVolume("pv1"),
			want:           false,
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create all the above resources and get a k8s client.
			var objects []client.Object
			for _, sc := range tt.storageClasses {
				objects = append(objects, sc)
			}
			objects = append(objects, &tt.pvc)
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			got, err := IsProvisionedVolume(client, tt.volume, testNamespace, DriverName)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsProvisionedVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsProvisionedVolume() = %v, want %v", got, tt.want)
			}
		})
	}
}

// createPVC creates and returns a PVC object.
func createPVC(name, namespace, storageClassName string, betaAnnotation bool) corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: make(map[string]string),
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
		pvc.ObjectMeta.Annotations[pvcStorageClassKey] = storageClassName
	} else {
		pvc.Spec.StorageClassName = &storageClassName
	}

	return pvc
}

// createVolume creates and returns a Volume object.
func createVolume(pvcName string) corev1.Volume {
	return corev1.Volume{
		Name: pvcName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
}
