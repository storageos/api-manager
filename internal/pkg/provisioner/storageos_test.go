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
				t.Errorf("IsStorageOSNode() error = %v, wantErr %t", gotErr, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsStorageOSNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasStorageOSAnnotation(t *testing.T) {
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
			if got := HasStorageOSAnnotation(tt.obj); got != tt.want {
				t.Errorf("HasStorageOSAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsProvisionedVolume(t *testing.T) {
	t.Parallel()

	provisioned, err := IsProvisionedVolume(nil, corev1.Volume{}, "")

	if provisioned {
		t.Errorf("IsProvisionedVolume() = %t", provisioned)
		return
	}
	if err != nil {
		t.Errorf("IsProvisionedVolume() error = %v, not allowed", err)
		return
	}
}

func TestIsProvisionedPVC(t *testing.T) {
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
		want           bool
		wantErr        bool
	}{
		{
			name:           "storageos volume",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, stosSC.Name, false),
			want:           true,
		},
		{
			name:           "storageos volume, beta annotation",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, stosSC.Name, true),
			want:           true,
		},
		{
			name:           "storageos volume, default storage class",
			storageClasses: []*storagev1.StorageClass{stosSCdefault, fooSC},
			pvc:            createPVC("pv1", testNamespace, "", false),
			want:           true,
		},
		{
			name:           "non-storageos volume",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, fooSC.Name, false),
			want:           false,
		},
		{
			name:           "non-storageos volume, beta annotation",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, fooSC.Name, true),
			want:           false,
		},
		{
			name:           "non-storageos volume, default storage class",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSCdefault},
			pvc:            createPVC("pv1", testNamespace, "", true),
			want:           false,
		},
		{
			name:           "non-storageos volume, no storage classes",
			storageClasses: []*storagev1.StorageClass{},
			pvc:            createPVC("pv1", testNamespace, "", true),
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

			got, err := IsProvisionedPVC(client, tt.pvc, testNamespace, DriverName)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsProvisionedPVC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsProvisionedPVC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsProvisionedStorageClass(t *testing.T) {
	provisioner := DriverName

	testcases := []struct {
		name            string
		storageClass    storagev1.StorageClass
		wantProvisioned bool
	}{
		{
			name:         "not-provisioned",
			storageClass: storagev1.StorageClass{},
		},
		{
			name: "provisioned",
			storageClass: storagev1.StorageClass{
				Provisioner: provisioner,
			},
			wantProvisioned: true,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			isProvisioned := IsProvisionedStorageClass(&tc.storageClass, provisioner)

			if isProvisioned != tc.wantProvisioned {
				t.Errorf("provisoned flag got:\n%t\n, want:\n%t", isProvisioned, tc.wantProvisioned)
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
