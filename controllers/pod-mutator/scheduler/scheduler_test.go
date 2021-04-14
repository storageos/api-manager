package scheduler

import (
	"context"
	"fmt"
	"testing"

	"github.com/storageos/api-manager/internal/pkg/provisioner"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMutatePodFn(t *testing.T) {
	// Test values only.
	storageosSchedulerName := "storageos-scheduler"
	storageosProvisioner := provisioner.DriverName
	defaultSchedulerName := "default-scheduler"
	schedulerAnnotationKey := "storageos.com/scheduler"

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
		Provisioner: storageosProvisioner,
	}

	// Non-StorageOS StorageClass.
	fooSC := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slow",
		},
		Provisioner: "foo-provisioner",
	}

	testNamespace := "default"

	// PVC that uses StorageOS StorageClass.
	stosPVC := createPVC("pv1", testNamespace, stosSC.Name, false)

	// PVC that uses non-StorageOS StorageClass.
	fooPVC := createPVC("pv3", testNamespace, fooSC.Name, false)

	testcases := []struct {
		name              string
		annotations       map[string]string
		volumeClaimNames  []string
		schedulerName     string
		wantSchedulerName string
	}{
		{
			name:              "pod with storageos volume",
			volumeClaimNames:  []string{stosPVC.Name},
			schedulerName:     storageosSchedulerName,
			wantSchedulerName: storageosSchedulerName,
		},
		{
			name:              "pod with storageos volume and scheduler disabled",
			volumeClaimNames:  []string{stosPVC.Name},
			schedulerName:     "",
			wantSchedulerName: defaultSchedulerName,
		},
		{
			name:              "pod without storageos volume",
			volumeClaimNames:  []string{fooPVC.Name},
			schedulerName:     storageosSchedulerName,
			wantSchedulerName: defaultSchedulerName,
		},
		{
			name:              "pod without storageos volume and scheduler disabled",
			volumeClaimNames:  []string{fooPVC.Name},
			schedulerName:     "",
			wantSchedulerName: defaultSchedulerName,
		},
		{
			name:              "pod with mixed volumes and scheduler disabled",
			volumeClaimNames:  []string{stosPVC.Name, fooPVC.Name},
			schedulerName:     "",
			wantSchedulerName: defaultSchedulerName,
		},
		{
			name: "pod with scheduler annotation false",
			annotations: map[string]string{
				schedulerAnnotationKey: "false",
			},
			volumeClaimNames:  []string{stosPVC.Name},
			schedulerName:     storageosSchedulerName,
			wantSchedulerName: defaultSchedulerName,
		},
		{
			name: "pod with scheduler annotation true",
			annotations: map[string]string{
				schedulerAnnotationKey: "true",
			},
			volumeClaimNames:  []string{stosPVC.Name},
			schedulerName:     storageosSchedulerName,
			wantSchedulerName: storageosSchedulerName,
		},
		{
			name: "pod with scheduler annotation invalid value",
			annotations: map[string]string{
				schedulerAnnotationKey: "foo",
			},
			volumeClaimNames:  []string{stosPVC.Name},
			schedulerName:     storageosSchedulerName,
			wantSchedulerName: storageosSchedulerName,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Pod that uses PVCs.
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod1",
					Namespace:   testNamespace,
					Annotations: tc.annotations,
				},
				Spec: corev1.PodSpec{
					SchedulerName: defaultSchedulerName,
					Volumes:       []corev1.Volume{},
					Containers: []corev1.Container{
						{
							Name:  "some-app",
							Image: "some-image",
						},
					},
				},
			}

			// Append the volumes in the pod spec.
			for i, claimName := range tc.volumeClaimNames {
				pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
					Name: fmt.Sprintf("vol%d", i),
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: claimName,
						},
					},
				})
			}

			// Create all the above resources and get a k8s client.
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stosSC, fooSC, stosPVC, fooPVC, pod).Build()

			// Create a PodSchedulerSetter instance with the fake client.
			podSchedulerSetter := PodSchedulerSetter{
				Client: client,
				Provisioners: []string{
					storageosProvisioner,
				},
				SchedulerName:          tc.schedulerName,
				SchedulerAnnotationKey: schedulerAnnotationKey,
				log:                    ctrl.Log,
			}

			// Pass the created pod to the mutatePodFn and check if the schedulerName in
			// podSpec changed.
			if err := podSchedulerSetter.MutatePod(context.Background(), pod, testNamespace); err != nil {
				t.Fatalf("failed to mutate pod: %v", err)
			}

			if pod.Spec.SchedulerName != tc.wantSchedulerName {
				t.Errorf("unexpected pod scheduler name:\n\t(WNT) %s\n\t(GOT) %s", tc.wantSchedulerName, pod.Spec.SchedulerName)
			}
		})
	}
}

// createPVC creates and returns a PVC object.
func createPVC(name, namespace, storageClassName string, betaAnnotation bool) *corev1.PersistentVolumeClaim {
	scAnnotationKey := "volume.beta.kubernetes.io/storage-class"

	pvc := &corev1.PersistentVolumeClaim{
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
		pvc.ObjectMeta.Annotations[scAnnotationKey] = storageClassName
	} else {
		pvc.Spec.StorageClassName = &storageClassName
	}

	return pvc
}
