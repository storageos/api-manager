package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/darkowlzz/operator-toolkit/webhook/cert"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	podmutator "github.com/storageos/api-manager/controllers/pod-mutator"
	"github.com/storageos/api-manager/controllers/pod-mutator/scheduler"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
)

// SetupSchedulerTest will set up a testing environment.  It must be called
// from each test.
func SetupSchedulerTest(ctx context.Context, schedulerName string, scs []*storagev1.StorageClass, pvcs []*corev1.PersistentVolumeClaim) {
	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		for _, sc := range scs {
			err := k8sClient.Create(ctx, sc)
			Expect(err).NotTo(HaveOccurred(), "failed to create test storageclass")
		}

		for _, pvc := range pvcs {
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "failed to create test pvc")
		}

		// Configure the certificate manager.
		certOpts := cert.Options{
			Service: &admissionv1.ServiceReference{
				Name:      webhookServiceName,
				Namespace: webhookServiceNamespace,
			},
			Client:                    k8sClient,
			SecretRef:                 &types.NamespacedName{Name: webhookSecretName, Namespace: webhookSecretNamespace},
			MutatingWebhookConfigRefs: []types.NamespacedName{{Name: webhookMutatingConfigName}},
		}

		err := cert.NewManager(nil, certOpts)
		Expect(err).NotTo(HaveOccurred(), "unable to provision certificate")

		webhookInstallOptions := &testEnv.WebhookInstallOptions
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Host:               webhookInstallOptions.LocalServingHost,
			Port:               webhookInstallOptions.LocalServingPort,
			CertDir:            webhookInstallOptions.LocalServingCertDir,
			LeaderElection:     false,
			MetricsBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		decoder, err := admission.NewDecoder(mgr.GetScheme())
		Expect(err).NotTo(HaveOccurred(), "failed to create decoder")

		podMutator := podmutator.NewController(mgr.GetClient(), decoder, []podmutator.Mutator{
			scheduler.NewPodSchedulerSetter(mgr.GetClient(), schedulerName),
		})
		mgr.GetWebhookServer().Register(webhookMutatePodsPath, &webhook.Admission{Handler: podMutator})
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		for _, pvc := range pvcs {
			err := k8sClient.Delete(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "failed to delete test pvc")
		}

		for _, sc := range scs {
			err := k8sClient.Delete(ctx, sc)
			Expect(err).NotTo(HaveOccurred(), "failed to delete test storageclass")
		}

		cancel()
	})
}

func genSC(prov string) storagev1.StorageClass {
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sc-" + randStringRunes(5),
		},
		Provisioner: prov,
	}
}

func genDefaultSC(prov string) storagev1.StorageClass {
	sc := genSC(prov)
	sc.SetAnnotations(map[string]string{
		provisioner.DefaultStorageClassKey: "true",
	})
	return sc
}

func genPVCWithStorageClass(scName string) corev1.PersistentVolumeClaim {
	pvc := genPVC()
	pvc.Spec.StorageClassName = &scName

	return pvc
}

func genPod(pvcNames ...string) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-" + randStringRunes(5),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{},
			Containers: []corev1.Container{
				{
					Name:         "test-app",
					Image:        "nginx",
					VolumeMounts: []corev1.VolumeMount{},
				},
			},
		},
	}
	for i, pvcName := range pvcNames {
		volName := fmt.Sprintf("some-data-%d", i+1)
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: fmt.Sprintf("/data-%d", i+1),
		})
	}

	return pod
}

func genPodWithAnnotation(pvcName string, schedulerAnnotationValue string) corev1.Pod {
	pod := genPod(pvcName)
	if schedulerAnnotationValue != "" {
		pod.SetAnnotations(map[string]string{
			scheduler.PodSchedulerAnnotationKey: schedulerAnnotationValue,
		})
	}
	return pod
}

var _ = Describe("Scheduler controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout              = time.Second * 2
		duration             = time.Second * 2
		interval             = time.Millisecond * 250
		defaultSchedulerName = "default-scheduler"
		testSchedulerName    = "test-scheduler"
	)

	ctx := context.Background()

	Context("When creating a Pod with a non-StorageOS PVC", func() {
		sc := genSC("not-storageos")
		pvc := genPVCWithStorageClass(sc.GetName())
		pod := genPod(pvc.GetName())
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should not set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler not to be set")
			Consistently(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(defaultSchedulerName))
		})
	})

	Context("When creating a Pod with a StorageOS PVC", func() {
		sc := genSC(provisioner.DriverName)
		pvc := genPVCWithStorageClass(sc.GetName())
		pod := genPod(pvc.GetName())
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler to be set")
			Eventually(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(testSchedulerName))
		})
	})

	Context("When creating a Pod with mixed PVCs", func() {
		sc1 := genSC("not-storageos")
		sc2 := genSC(provisioner.DriverName)
		pvc1 := genPVCWithStorageClass(sc1.GetName())
		pvc2 := genPVCWithStorageClass(sc2.GetName())
		pod := genPod(pvc1.GetName(), pvc2.GetName())
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc1, &sc2}, []*corev1.PersistentVolumeClaim{&pvc1, &pvc2})
		It("Should set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler to be set")
			Eventually(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(testSchedulerName))
		})
	})

	Context("When creating a Pod with a StorageOS PVC with the scheduler disabled", func() {
		sc := genSC(provisioner.DriverName)
		pvc := genPVCWithStorageClass(sc.GetName())
		pod := genPod(pvc.GetName())
		SetupSchedulerTest(ctx, "", []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should not set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler not to be set")
			Consistently(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(defaultSchedulerName))
		})
	})

	Context("When creating a Pod with a non-StorageOS PVC using a default StorageClass", func() {
		sc := genDefaultSC("not-storageos")
		pvc := genPVC()
		pod := genPod(pvc.GetName())
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should not set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler not to be set")
			Consistently(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(defaultSchedulerName))
		})
	})

	Context("When creating a Pod with a StorageOS PVC using a default StorageClass", func() {
		sc := genDefaultSC(provisioner.DriverName)
		pvc := genPVC()
		pod := genPod(pvc.GetName())
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler to be set")
			Eventually(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(testSchedulerName))
		})
	})

	Context("When creating a Pod with a StorageOS PVC and the Pod scheduler annotation set to false", func() {
		sc := genSC(provisioner.DriverName)
		pvc := genPVCWithStorageClass(sc.GetName())
		pod := genPodWithAnnotation(pvc.GetName(), "false")
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should not set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler not to be set")
			Consistently(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(defaultSchedulerName))
		})
	})

	Context("When creating a Pod with a StorageOS PVC and the Pod scheduler annotation set to true", func() {
		sc := genSC(provisioner.DriverName)
		pvc := genPVCWithStorageClass(sc.GetName())
		pod := genPodWithAnnotation(pvc.GetName(), "true")
		SetupSchedulerTest(ctx, testSchedulerName, []*storagev1.StorageClass{&sc}, []*corev1.PersistentVolumeClaim{&pvc})
		It("Should set the StorageOS scheduler", func() {
			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod scheduler to be set")
			Eventually(func() string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return ""
				}
				return got.Spec.SchedulerName
			}, timeout, interval).Should(Equal(testSchedulerName))
		})
	})

})
