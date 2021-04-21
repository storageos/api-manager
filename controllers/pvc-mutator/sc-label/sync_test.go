package sclabel

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMutatePVC(t *testing.T) {
	// Create a new scheme and add all the types from different clientsets.
	scheme := runtime.NewScheme()
	if err := kscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	longName := strings.Join(make([]string, validation.LabelValueMaxLength+2), "X")

	// StorageOS StorageClass.
	stosSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "stos",
		},
		Provisioner: provisioner.DriverName,
		Parameters: map[string]string{
			"non-reserved":                                 "need-to-skip",
			storageos.ReservedLabelK8sPVCNamespace:         "need-to-skip",
			storageos.ReservedLabelK8sPVCName:              "need-to-skip",
			storageos.ReservedLabelK8sPVName:               "need-to-skip",
			storageos.ReservedLabelPrefix + "_invalid-key": "need-to-skip",
			storageos.ReservedLabelPrefix + "need-to-skip": longName,
			storageos.ReservedLabelPrefix + "param":        "value",
		},
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
		name       string
		namespace  string
		notStos    bool
		labels     map[string]string
		wantLabels map[string]string
		wantErr    bool
	}{
		{
			name:      "foreign pvc",
			namespace: testNamespace,
			notStos:   true,
		},
		{
			name:       "nil labels",
			namespace:  testNamespace,
			labels:     nil,
			wantLabels: map[string]string{storageos.ReservedLabelPrefix + "param": "value"},
		},
		{
			name:       "overwrite label",
			namespace:  testNamespace,
			labels:     map[string]string{storageos.ReservedLabelPrefix + "param": "overwrited"},
			wantLabels: map[string]string{storageos.ReservedLabelPrefix + "param": "overwrited"},
		},
		{
			name:      "has extra label",
			namespace: testNamespace,
			labels:    map[string]string{"extra": "value"},
			wantLabels: map[string]string{
				storageos.ReservedLabelPrefix + "param": "value",
				"extra":                                 "value",
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Create all the above resources and get a k8s client.
			k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stosSC, notstosSC).Build()

			// Create a LabelSetter instance with the fake client.
			labelSetter := LabelSetter{
				Client: k8s,
				log:    ctrl.Log,
			}

			scName := stosSC.Name
			if tc.notStos {
				scName = notstosSC.Name
			}

			pvc := createPVC("pvc1", tc.namespace, scName, tc.labels)

			err := labelSetter.MutatePVC(context.Background(), pvc, tc.namespace)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("got unexpected error: %v", err)
				}
				return
			}

			// Check labels on pvc
			if tc.wantLabels != nil {
				if !reflect.DeepEqual(tc.wantLabels, pvc.Labels) {
					t.Errorf("labes must match got:\n%v\n, want:\n%v", pvc.Labels, tc.wantLabels)
				}
			}
		})
	}
}

// createPVC creates and returns a PVC object.
func createPVC(name, namespace, storageClassName string, labels map[string]string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
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
			StorageClassName: &storageClassName,
		},
	}
}
