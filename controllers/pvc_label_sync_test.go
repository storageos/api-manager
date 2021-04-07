package controllers

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pvclabel "github.com/storageos/api-manager/controllers/pvc-label"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// SetupPVCLabelSyncTest will set up a testing environment.  It must be called
// from each test.
func SetupPVCLabelSyncTest(ctx context.Context, isStorageOS bool, createLabels map[string]string, gcEnabled bool) client.ObjectKey {
	var ns *corev1.Namespace
	var pvc *corev1.PersistentVolumeClaim
	var cancel func()

	rand := randStringRunes(5)
	var key = client.ObjectKey{
		Name:      "pvc-" + rand,
		Namespace: "ns-" + rand,
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: key.Namespace,
			},
		}
		err := k8sClient.Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

		pvName := "pvc-" + randStringRunes(5)
		volumeMode := v1.PersistentVolumeFilesystem
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels:    createLabels,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{v1.PersistentVolumeAccessMode("ReadWriteOnce")},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				VolumeMode: &volumeMode,
				VolumeName: pvName,
			},
		}
		if isStorageOS {
			pvc.Annotations = map[string]string{
				provisioner.PVCProvisionerAnnotationKey: provisioner.DriverName,
			}
		}

		err = k8sClient.Create(ctx, pvc)
		Expect(err).NotTo(HaveOccurred(), "failed to create test pvc")

		api = storageos.NewMockClient()
		vol := storageos.MockObject{
			Name:      pvName,
			Namespace: pvc.GetNamespace(),
			Labels:    pvc.GetLabels(),
		}
		err = api.AddVolume(vol)
		Expect(err).NotTo(HaveOccurred(), "failed to create test volume in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		gcInterval := defaultSyncInterval
		if gcEnabled {
			gcInterval = time.Hour
		}

		controller := pvclabel.NewReconciler(api, mgr.GetClient(), defaultSyncDelay, gcInterval)
		err = controller.SetupWithManager(mgr, defaultWorkers)
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, pvc)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test pvc")
		err = k8sClient.Delete(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test namespace")
		cancel()
	})

	return key
}

var _ = Describe("PVC Label controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var unreservedLabels = map[string]string{
		"foo": "bar",
		"baz": "boo",
	}
	var reservedLabels = map[string]string{
		storageos.ReservedLabelReplicas: "1",
	}
	var mixedLabels = map[string]string{
		"foo":                           "bar",
		"baz":                           "boo",
		storageos.ReservedLabelReplicas: "1",
	}

	ctx := context.Background()

	Context("When adding unreserved labels", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			volKey := client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()}
			By("By adding unreserved labels to k8s PVC")
			pvc.SetLabels(unreservedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(volKey)
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(unreservedLabels))
		})
	})

	Context("When adding reserved labels", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("By adding reserved labels to k8s PVC")
			pvc.SetLabels(reservedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(reservedLabels))
		})
	})

	Context("When adding mixed labels", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("By adding mixed labels to k8s PVC")
			pvc.SetLabels(mixedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(mixedLabels))
		})
	})

	Context("When adding unrecognised reserved labels", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should only sync recognised labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("By adding unrecognised reserved labels to k8s PVC")
			labels := map[string]string{}
			for k, v := range unreservedLabels {
				labels[k] = v
			}
			labels[storageos.ReservedLabelPrefix+"unrecognised"] = "true"
			pvc.SetLabels(labels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match other labels")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(unreservedLabels))
		})
	})

	Context("When adding replicas label", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("By adding replicas label to k8s PVC")
			labels := map[string]string{
				storageos.ReservedLabelReplicas: "1",
			}
			pvc.SetLabels(labels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(labels))
		})
	})

	Context("When adding and removing mixed labels", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("By adding labels to k8s PVC")
			pvc.SetLabels(mixedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(mixedLabels))

			By("By removing labels from k8s PVC")
			pvc.SetLabels(map[string]string{})
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When adding replicas label and the StorageOS API returns an error", func() {
		key := SetupPVCLabelSyncTest(ctx, true, nil, false)
		It("Should not sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("Setting API to return error")
			api.EnsureVolumeLabelsErr = errors.New("fake error")

			By("By adding replicas label to k8s PVC")
			labels := map[string]string{
				storageos.ReservedLabelReplicas: "1000",
			}
			pvc.SetLabels(labels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels to be empty")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When adding labels on a k8s PVC not provisioned by StorageOS", func() {
		key := SetupPVCLabelSyncTest(ctx, false, nil, false)
		It("Should not sync labels to StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("By adding labels to k8s PVC")
			pvc.SetLabels(unreservedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Volume labels not to match")
			Consistently(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, duration, interval).ShouldNot(Equal(unreservedLabels))
		})
	})

	Context("When starting after a k8s PVC with labels has been created", func() {
		key := SetupPVCLabelSyncTest(ctx, true, mixedLabels, true)
		It("The resync should update the StorageOS Volume", func() {
			By("Confirming PVC exists in k8s")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, key, &pvc)).Should(Succeed())

			By("Expecting StorageOS Volume labels to match")
			Eventually(func() map[string]string {
				vol, err := api.GetVolume(client.ObjectKey{Name: pvc.Spec.VolumeName, Namespace: pvc.GetNamespace()})
				if err != nil {
					return nil
				}
				return vol.GetLabels()
			}, timeout, interval).Should(Equal(mixedLabels))
		})
	})

})
