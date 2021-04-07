package provisioner

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStorageClassForPVC(t *testing.T) {
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
		want           *storagev1.StorageClass
		wantErr        bool
	}{
		{
			name:           "storageos volume",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, stosSC.Name, false),
			want:           stosSC,
		},
		{
			name:           "storageos volume, beta annotation",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, stosSC.Name, true),
			want:           stosSC,
		},
		{
			name:           "storageos volume, default storage class",
			storageClasses: []*storagev1.StorageClass{stosSCdefault, fooSC},
			pvc:            createPVC("pv1", testNamespace, "", false),
			want:           stosSCdefault,
		},
		{
			name:           "non-storageos volume",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, fooSC.Name, false),
			want:           fooSC,
		},
		{
			name:           "non-storageos volume, beta annotation",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSC},
			pvc:            createPVC("pv1", testNamespace, fooSC.Name, true),
			want:           fooSC,
		},
		{
			name:           "non-storageos volume, default storage class",
			storageClasses: []*storagev1.StorageClass{stosSC, fooSCdefault},
			pvc:            createPVC("pv1", testNamespace, "", true),
			want:           fooSCdefault,
		},
		{
			name:           "non-storageos volume, no storage classes",
			storageClasses: []*storagev1.StorageClass{},
			pvc:            createPVC("pv1", testNamespace, "", true),
			want:           nil,
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
			k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			got, err := StorageClassForPVC(k8s, &tt.pvc)
			if (err != nil) != tt.wantErr {
				t.Errorf("StorageClassForPVC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil && err == nil {
				t.Error("StorageClassForPVC() returned nil and no error")
			}
			if got != nil {
				gKey := client.ObjectKeyFromObject(got)
				wKey := client.ObjectKeyFromObject(tt.want)
				if !reflect.DeepEqual(gKey, wKey) {
					t.Errorf("StorageClassForPVC() = %s, want %s", gKey, wKey)
				}
			}
		})
	}
}

func TestPVCStorageClassName(t *testing.T) {
	t.Parallel()

	testNamespace := "default"
	testSCName := "test-sc"

	tests := []struct {
		name string
		pvc  corev1.PersistentVolumeClaim
		want string
	}{
		{
			name: "StorageClassName",
			pvc:  createPVC("pv1", testNamespace, testSCName, false),
			want: testSCName,
		},
		{
			name: "beta annotation",
			pvc:  createPVC("pv1", testNamespace, testSCName, true),
			want: testSCName,
		},
		{
			name: "empty",
			pvc:  createPVC("pv1", testNamespace, "", true),
			want: "",
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := PVCStorageClassName(&tt.pvc); got != tt.want {
				t.Errorf("PVCStorageClassName() = %v, want %v", got, tt.want)
			}
		})
	}
}
