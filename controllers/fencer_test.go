package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	storageosv1 "github.com/storageos/api-manager/api/v1"
	fencer "github.com/storageos/api-manager/controllers/fencer"
	"github.com/storageos/api-manager/internal/pkg/annotation"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

const (
	// defaultPollInterval is how often nodes are polled in the StorageOS API.
	defaultPollInterval = 1 * time.Second

	// defaultExpiryInterval is how often nodes are expired from the cache. This
	// determines how often Pods on an unhealthy node are re-evaluated for
	// fencing.
	defaultExpiryInterval = 2 * time.Millisecond
)

// SetupFencerTest will set up a testing environment.  It must be called
// from each test.
func SetupFencerTest(ctx context.Context, nsName string, node *corev1.Node, pod *corev1.Pod, pvcs []*corev1.PersistentVolumeClaim, pvs []*corev1.PersistentVolume, vas []*storagev1.VolumeAttachment, vols []storageos.Object) {
	var cancel func()
	var errCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-errCh:
				// read and ignore so sender doesn't block.
			case <-ctx.Done():
				return
			}
		}
	}()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)
		api = storageos.NewMockClient()

		// Create node.
		err := k8sClient.Create(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in k8s")

		err = api.AddNode(storageos.MockObject{Name: node.Name, Healthy: true})
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		// Create namespace.
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: nsName},
		}
		err = k8sClient.Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace in k8s")

		// Create PVCs.
		for _, pvc := range pvcs {
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pvc in k8s")
		}

		// Create PVs.
		for _, pv := range pvs {
			err := k8sClient.Create(ctx, pv)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pv in k8s")
		}

		// Create VolumeAttachments.
		for _, va := range vas {
			err := k8sClient.Create(ctx, va)
			Expect(err).NotTo(HaveOccurred(), "failed to create test va in k8s")
		}

		// Create Volumes.
		for _, vol := range vols {
			err := api.AddVolume(vol)
			Expect(err).NotTo(HaveOccurred(), "failed to create test volume in storageos")
		}

		// Create Pod.
		err = k8sClient.Create(ctx, pod)
		Expect(err).NotTo(HaveOccurred(), "failed to create test pod in k8s")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		err = storageosv1.AddToScheme(mgr.GetScheme())
		Expect(err).NotTo(HaveOccurred(), "failed to add scheme")

		controller := fencer.NewReconciler(api, errCh, mgr.GetClient(), defaultPollInterval, defaultExpiryInterval)
		err = controller.SetupWithManager(ctx, mgr, defaultWorkers)
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		// Don't delete the node, the test should have.
		// Stop the manager.
		cancel()
	})
}

func genNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func genPod(name string, namespace string, fenced bool, nodeName string, pvcs []*corev1.PersistentVolumeClaim) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "some-app",
					Image: "some-image",
				},
			},
			NodeName: nodeName,
		},
	}
	if fenced {
		pod.Labels[storageos.ReservedLabelFencing] = "true"
	}
	for i, pvc := range pvcs {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: fmt.Sprintf("vol%d", i),
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		})
	}
	return pod
}
func genPVC(name string, namespace string, volName string, stos bool) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{},
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
			VolumeName: volName,
		},
	}
	if stos {
		pvc.Annotations[annotation.ProvisionerAnnotationKey] = fencer.DriverName
	}
	return pvc
}

func genPV(volName string, driverName string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: volName, // doesn't need to match storageos volume.
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "", // must be empty to match pvc.
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       driverName,
					VolumeHandle: volName, // must be same name as storageos volume.
				},
			},
		},
	}
}

func genVA(vaName string, volName string, nodeName string) *storagev1.VolumeAttachment {
	return &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: vaName,
		},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: fencer.DriverName,
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: &volName,
			},
			NodeName: nodeName,
		},
	}
}

func genVol(name string, namespace string, healthy bool) *storageos.MockObject {
	return &storageos.MockObject{
		Name:      name,
		Namespace: namespace,
		Healthy:   healthy,
	}
}

func genNames(suffix string) (nsName, nodeName, podName, volName, pvcName, vaName string) {
	nsName = "ns-" + suffix
	nodeName = "node-" + suffix
	podName = "pod-" + suffix
	volName = "stos-" + suffix
	pvcName = "pvc-" + suffix
	vaName = "va-" + suffix
	return
}

func key(name string, namespace string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: namespace}
}

var _ = Describe("Fencing controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 5
		duration = time.Second * 2
		interval = time.Millisecond * 250
	)

	ctx := context.Background()

	Context("Pod with fencing and storageos pvc", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{genPVC(pvcName, nsName, volName, true)}
		vas := []*storagev1.VolumeAttachment{genVA(vaName, volName, nodeName)}
		vols := []storageos.Object{genVol(volName, nsName, true)}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachment exists")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName, ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting VolumeAttachment to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(vaName, ""), &va)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Pod with fencing and pre-provisioned storageos pv", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		scName := ""
		pvcs := []*corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:        pvcName,
					Namespace:   nsName,
					Annotations: map[string]string{},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &scName, // must be empty or default will be used.
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
					VolumeName: volName, // must be same name as PV name.
				},
			},
		}
		vas := []*storagev1.VolumeAttachment{genVA(vaName, volName, nodeName)}
		vols := []storageos.Object{genVol(volName, nsName, true)}
		pod := genPod(podName, nsName, true, nodeName, pvcs)
		pvs := []*corev1.PersistentVolume{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      volName,
					Namespace: nsName,
				},
				Spec: corev1.PersistentVolumeSpec{
					StorageClassName: scName, // must be empty to match pvc.
					Capacity: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       fencer.DriverName,
							VolumeHandle: volName, // must be same name as storageos volume.
						},
					},
				},
			},
		}

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, pvs, vas, vols)
		It("Should delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachment exists")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName, ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting VolumeAttachment to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(vaName, ""), &va)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Pod without fencing and storageos pvc", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{genPVC(pvcName, nsName, volName, true)}
		vas := []*storagev1.VolumeAttachment{genVA(vaName, volName, nodeName)}
		vols := []storageos.Object{genVol(volName, nsName, true)}
		pod := genPod(podName, nsName, false, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should not delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachment exists")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName, ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, duration, interval).Should(Succeed())

			By("Expecting VolumeAttachment to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName, ""), &va)
			}, duration, interval).Should(Succeed())
		})
	})

	Context("Pod with fencing and unhealthy storageos pvc", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{genPVC(pvcName, nsName, volName, true)}
		vas := []*storagev1.VolumeAttachment{genVA(vaName, volName, nodeName)}
		vols := []storageos.Object{genVol(volName, nsName, true)}
		pod := genPod(podName, nsName, false, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should not delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachment exists")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName, ""), &va)).Should(Succeed())

			By("By marking storageos volume unhealthy")
			Expect(api.UpdateVolumeHealth(key(volName, nsName), false)).Should(BeTrue())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, duration, interval).Should(Succeed())

			By("Expecting VolumeAttachment to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName, ""), &va)
			}, duration, interval).Should(Succeed())
		})
	})

	Context("Pod with fencing and no storageos pvc", func() {
		nsName, nodeName, podName, _, _, _ := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{}
		vas := []*storagev1.VolumeAttachment{}
		vols := []storageos.Object{}
		pod := genPod(podName, nsName, false, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should not delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, duration, interval).Should(Succeed())
		})
	})

	Context("Pod with fencing and multiple storageos pvcs", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{
			genPVC(pvcName+"-1", nsName, volName+"-1", true),
			genPVC(pvcName+"-2", nsName, volName+"-2", true),
			genPVC(pvcName+"-3", nsName, volName+"-3", true),
		}
		vas := []*storagev1.VolumeAttachment{
			genVA(vaName+"-1", volName+"-1", nodeName),
			genVA(vaName+"-2", volName+"-2", nodeName),
			genVA(vaName+"-3", volName+"-3", nodeName),
		}
		vols := []storageos.Object{
			genVol(volName+"-1", nsName, true),
			genVol(volName+"-2", nsName, true),
			genVol(volName+"-3", nsName, true),
		}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachments exist")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName+"-1", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-2", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-3", ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting VolumeAttachment to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(va.Name, va.Namespace), &va)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Pod with fencing and multiple storageos pvcs with one unhealthy", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{
			genPVC(pvcName+"-1", nsName, volName+"-1", true),
			genPVC(pvcName+"-2", nsName, volName+"-2", true),
			genPVC(pvcName+"-3", nsName, volName+"-3", true),
		}
		vas := []*storagev1.VolumeAttachment{
			genVA(vaName+"-1", volName+"-1", nodeName),
			genVA(vaName+"-2", volName+"-2", nodeName),
			genVA(vaName+"-3", volName+"-3", nodeName),
		}
		vols := []storageos.Object{
			genVol(volName+"-1", nsName, true),
			genVol(volName+"-2", nsName, true),
			genVol(volName+"-3", nsName, false), // unhealthy
		}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should not delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachments exist")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName+"-1", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-2", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-3", ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, duration, interval).Should(Succeed())

			By("Expecting VolumeAttachment 1 to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName+"-1", ""), &va)
			}, duration, interval).Should(Succeed())
			By("Expecting VolumeAttachment 2 to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName+"-2", ""), &va)
			}, duration, interval).Should(Succeed())
			By("Expecting VolumeAttachment 3 to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName+"-3", ""), &va)
			}, duration, interval).Should(Succeed())

		})
	})

	Context("Pod with fencing and mixed pvcs", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{
			genPVC(pvcName+"-1", nsName, volName+"-1", true),
			genPVC(pvcName+"-2", nsName, volName+"-2", false), // not StorageOS
			genPVC(pvcName+"-3", nsName, volName+"-3", true),
		}
		pvs := []*corev1.PersistentVolume{genPV(volName+"-2", "csi.vendor-x.com")}
		vas := []*storagev1.VolumeAttachment{
			genVA(vaName+"-1", volName+"-1", nodeName),
			genVA(vaName+"-2", "non-stos", nodeName),
			genVA(vaName+"-3", volName+"-3", nodeName),
		}
		vols := []storageos.Object{
			genVol(volName+"-1", nsName, true),
			genVol(volName+"-3", nsName, true),
		}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, pvs, vas, vols)
		It("Should not delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachments exist")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName+"-1", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-2", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-3", ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting StorageOS VolumeAttachments to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(vaName+"-1", ""), &va)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting non-StorageOS VolumeAttachments to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName+"-2", ""), &va)
			}, duration, interval).Should(Succeed())

			By("Expecting StorageOS VolumeAttachments to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(vaName+"-3", ""), &va)
			}, timeout, interval).ShouldNot(Succeed())

		})
	})

	// Adding this test as it was a conscious choice to allow a failure to find
	// a statically-provisioned PV to not block fencing the Pod.
	//
	// The assumption is that it's a non-StorageOS volume since it can't be
	// attached to the pod without the PV.  It also covers cases where a PVC is
	// used with a non-CSI VolumeSource, e.g. ephemeral volumes, secrets, hostpath
	// that we may not have tested all permutations of.
	Context("Pod with fencing and mixed pvcs with missing pv", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{
			genPVC(pvcName+"-1", nsName, volName+"-1", true),
			genPVC(pvcName+"-2", nsName, volName+"-2", false), // not StorageOS, no PV
			genPVC(pvcName+"-3", nsName, volName+"-3", true),
		}
		// pvs := []*corev1.PersistentVolume{genPV(volName+"-2", "csi.vendor-x.com")}
		vas := []*storagev1.VolumeAttachment{
			genVA(vaName+"-1", volName+"-1", nodeName),
			genVA(vaName+"-2", "non-stos", nodeName),
			genVA(vaName+"-3", volName+"-3", nodeName),
		}
		vols := []storageos.Object{
			genVol(volName+"-1", nsName, true),
			genVol(volName+"-3", nsName, true),
		}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should not delete the Pod when its node is unhealthy", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("Confirming VolumeAttachments exist")
			var va storagev1.VolumeAttachment
			Expect(k8sClient.Get(ctx, key(vaName+"-1", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-2", ""), &va)).Should(Succeed())
			Expect(k8sClient.Get(ctx, key(vaName+"-3", ""), &va)).Should(Succeed())

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting StorageOS VolumeAttachments to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(vaName+"-1", ""), &va)
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting non-StorageOS VolumeAttachments to remain")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(vaName+"-2", ""), &va)
			}, duration, interval).Should(Succeed())

			By("Expecting StorageOS VolumeAttachments to be deleted")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(vaName+"-3", ""), &va)
			}, timeout, interval).ShouldNot(Succeed())

		})
	})

	Context("Pod with fencing and StorageOS api list nodes failed", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{genPVC(pvcName, nsName, volName, true)}
		vas := []*storagev1.VolumeAttachment{genVA(vaName, volName, nodeName)}
		vols := []storageos.Object{genVol(volName, nsName, true)}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should delete the Pod when its node is unhealthy and the api recovers", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("By forcing storageos api to fail to list nodes")
			api.ListNodesErr = errors.New("not now")

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod not to be deleted while api erroring")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, duration, interval).Should(Succeed())

			By("By allowing storageos api to list nodes")
			api.ListNodesErr = nil

			By("Expecting Pod to be deleted after list nodes fixed")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})

	Context("Pod with fencing and StorageOS api get volume failed", func() {
		nsName, nodeName, podName, volName, pvcName, vaName := genNames(randStringRunes(5))
		pvcs := []*corev1.PersistentVolumeClaim{genPVC(pvcName, nsName, volName, true)}
		vas := []*storagev1.VolumeAttachment{genVA(vaName, volName, nodeName)}
		vols := []storageos.Object{genVol(volName, nsName, true)}
		pod := genPod(podName, nsName, true, nodeName, pvcs)

		SetupFencerTest(ctx, nsName, genNode(nodeName), pod, pvcs, nil, vas, vols)
		It("Should delete the Pod when its node is unhealthy and the api recovers", func() {
			By("Confirming Pod exists in k8s on node")
			var gotPod corev1.Pod
			Expect(k8sClient.Get(ctx, key(podName, nsName), &gotPod)).Should(Succeed())
			Expect(pod.Spec.NodeName).Should(Equal(nodeName))

			By("By forcing storageos api to fail to get volume")
			api.GetVolumeErr = errors.New("not now")

			By("By marking storageos node unhealthy")
			Expect(api.UpdateNodeHealth(key(nodeName, ""), false)).Should(BeTrue())

			By("Expecting Pod not to be deleted while api erroring")
			Consistently(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, duration, interval).Should(Succeed())

			By("By allowing storageos api to get volume")
			api.GetVolumeErr = nil

			By("Expecting Pod to be deleted after get volume fixed")
			Eventually(func() error {
				return k8sClient.Get(ctx, key(podName, nsName), &gotPod)
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
