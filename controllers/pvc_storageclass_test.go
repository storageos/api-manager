package controllers

import (
	"context"
	"time"

	cclient "github.com/darkowlzz/operator-toolkit/client/composite"
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

	pvcmutator "github.com/storageos/api-manager/controllers/pvc-mutator"
	"github.com/storageos/api-manager/controllers/pvc-mutator/storageclass"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
)

// Define utility constants for object names and testing timeouts and intervals.
const (
	storageClassTimeout  = time.Second * 1
	storageClassInterval = time.Millisecond * 250
)

var (
	stosDefaultStorageClassName    = "stos-default"
	stosStorageClassName           = "stos"
	nonStosDefaultStorageClassName = "non-stos-default"
	nonStosStorageClassName        = "non-stos"
)

// SetupPVCStorageClassAnnotationTest will set up a testing environment.  It must be called
// from each test.
func SetupPVCStorageClassAnnotationTest(ctx context.Context, storageClasses ...storagev1.StorageClass) {
	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

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

		compositeClient, err := cclient.NewClientFromManager(mgr, cclient.Options{})
		if err != nil {
			Expect(err).NotTo(HaveOccurred(), "failed to create composite client")
			return
		}

		decoder, err := admission.NewDecoder(mgr.GetScheme())
		Expect(err).NotTo(HaveOccurred(), "failed to create decoder")

		pvcMutator := pvcmutator.NewController(compositeClient, decoder, []pvcmutator.Mutator{
			storageclass.NewAnnotationSetter(compositeClient),
		})

		mgr.GetWebhookServer().Register(webhookMutatePVCsPath, &webhook.Admission{Handler: pvcMutator})
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		for _, sc := range storageClasses {
			sc := sc
			Expect(k8sClient.Create(ctx, &sc)).Should(Succeed())
		}

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		for _, sc := range storageClasses {
			sc := sc
			Expect(k8sClient.Delete(ctx, &sc)).Should(Succeed())
		}

		cancel()
	})
}

var _ = Describe("The default StorageClass has not been configured", func() {
	ctx := context.Background()

	Context("When there is no given StorageClass", func() {
		SetupPVCStorageClassAnnotationTest(ctx)

		It("The pvc should not be created", func() {
			By("Expecting the PVC has default StorageClass")
			pvc := genPVC()

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).ShouldNot(Succeed())
		})
	})

	Context("When the given StorageClass is non StorageOS", func() {
		storageClass := getStorageClass(nonStosStorageClassName, false)

		SetupPVCStorageClassAnnotationTest(ctx, storageClass)

		It("The pvc should be created without annotation", func() {
			pvc := genPVC()
			pvc.Spec.StorageClassName = &storageClass.Name

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC has to be unchanged")
			Consistently(func() corev1.PersistentVolumeClaim {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return corev1.PersistentVolumeClaim{}
				}
				return got
			}, storageClassTimeout, storageClassInterval).Should(Equal(pvc))
		})
	})

	Context("When the given StorageClass is StorageOS", func() {
		storageClass := getStorageClass(stosStorageClassName, false)

		SetupPVCStorageClassAnnotationTest(ctx, storageClass)

		It("The pvc should be created with annotation", func() {
			pvc := genPVC()
			pvc.Spec.StorageClassName = &storageClass.Name

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Fetching the StorageClass")
			persistedSC := storagev1.StorageClass{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: storageClass.Name}, &persistedSC)
			Expect(err).NotTo(HaveOccurred(), "failed to fetch StorageClass")

			By("Expecting the PVC to be patched")
			Eventually(func() string {
				var mutatedPVC corev1.PersistentVolumeClaim

				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}

				return mutatedPVC.Annotations[provisioner.StorageClassUUIDAnnotationKey]
			}, storageClassTimeout, storageClassInterval).Should(Equal(string(persistedSC.UID)))
		})
	})
})

var _ = Describe("The default StorageClass is not StorageOS", func() {
	ctx := context.Background()

	Context("When there is no given StorageClass", func() {
		defaultStorageClass := getStorageClass(nonStosDefaultStorageClassName, true)

		SetupPVCStorageClassAnnotationTest(ctx, defaultStorageClass)

		It("The pvc should be created without annotation", func() {
			pvc := genPVC()

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC has to be unchanged")
			Consistently(func() corev1.PersistentVolumeClaim {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return corev1.PersistentVolumeClaim{}
				}
				return got
			}, storageClassTimeout, storageClassInterval).Should(Equal(pvc))
		})
	})

	Context("When the given StorageClass is not StorageOS", func() {
		defaultStorageClass := getStorageClass(nonStosDefaultStorageClassName, true)
		storageClass := getStorageClass(nonStosStorageClassName, false)

		SetupPVCStorageClassAnnotationTest(ctx, defaultStorageClass, storageClass)

		It("The pvc should be created without annotation", func() {
			pvc := genPVC()
			pvc.Spec.StorageClassName = &storageClass.Name

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC has to be unchanged")
			Consistently(func() corev1.PersistentVolumeClaim {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return corev1.PersistentVolumeClaim{}
				}
				return got
			}, storageClassTimeout, storageClassInterval).Should(Equal(pvc))
		})
	})

	Context("When the given StorageClass is StorageOS", func() {
		defaultStorageClass := getStorageClass(nonStosDefaultStorageClassName, true)
		storageClass := getStorageClass(stosStorageClassName, false)

		SetupPVCStorageClassAnnotationTest(ctx, defaultStorageClass, storageClass)

		It("The pvc should be created with annotation", func() {
			pvc := genPVC()
			pvc.Spec.StorageClassName = &storageClass.Name

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Fetching the StorageClass")
			persistedSC := storagev1.StorageClass{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: storageClass.Name}, &persistedSC)
			Expect(err).NotTo(HaveOccurred(), "failed to fetch StorageClass")

			By("Expecting the PVC to be patched")
			Eventually(func() string {
				var mutatedPVC corev1.PersistentVolumeClaim

				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}

				return mutatedPVC.Annotations[provisioner.StorageClassUUIDAnnotationKey]
			}, storageClassTimeout, storageClassInterval).Should(Equal(string(persistedSC.UID)))
		})
	})
})

var _ = Describe("The default StorageClass is StorageOS", func() {
	ctx := context.Background()

	Context("When there is no given StorageClass", func() {
		defaultStorageClass := getStorageClass(stosDefaultStorageClassName, true)

		SetupPVCStorageClassAnnotationTest(ctx, defaultStorageClass)

		It("The pvc should be created with annotation", func() {
			pvc := genPVC()

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Fetching the StorageClass")
			persistedSC := storagev1.StorageClass{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: defaultStorageClass.Name}, &persistedSC)
			Expect(err).NotTo(HaveOccurred(), "failed to fetch StorageClass")

			By("Expecting the PVC to be patched")
			Eventually(func() string {
				var mutatedPVC corev1.PersistentVolumeClaim

				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}

				return mutatedPVC.Annotations[provisioner.StorageClassUUIDAnnotationKey]
			}, storageClassTimeout, storageClassInterval).Should(Equal(string(persistedSC.UID)))
		})
	})

	Context("When the given StorageClass is not StorageOS", func() {
		defaultStorageClass := getStorageClass(stosDefaultStorageClassName, true)
		storageClass := getStorageClass(nonStosStorageClassName, false)

		SetupPVCStorageClassAnnotationTest(ctx, defaultStorageClass, storageClass)

		It("The pvc should be created without annotation", func() {
			pvc := genPVC()
			pvc.Spec.StorageClassName = &storageClass.Name

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC has to be unchanged")
			Consistently(func() corev1.PersistentVolumeClaim {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return corev1.PersistentVolumeClaim{}
				}
				return got
			}, storageClassTimeout, storageClassInterval).Should(Equal(pvc))
		})
	})

	Context("When the given StorageClass is StorageOS", func() {
		defaultStorageClass := getStorageClass(stosDefaultStorageClassName, true)
		storageClass := getStorageClass(stosStorageClassName, false)

		SetupPVCStorageClassAnnotationTest(ctx, defaultStorageClass, storageClass)

		It("The pvc should be created with annotation", func() {
			pvc := genPVC()
			pvc.Spec.StorageClassName = &storageClass.Name

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Fetching the StorageClass")
			persistedSC := storagev1.StorageClass{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: storageClass.Name}, &persistedSC)
			Expect(err).NotTo(HaveOccurred(), "failed to fetch StorageClass")

			By("Expecting the PVC to be patched")
			Eventually(func() string {
				var mutatedPVC corev1.PersistentVolumeClaim

				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}

				return mutatedPVC.Annotations[provisioner.StorageClassUUIDAnnotationKey]
			}, storageClassTimeout, storageClassInterval).Should(Equal(string(persistedSC.UID)))
		})
	})
})

func getStorageClass(name string, isDefault bool) storagev1.StorageClass {
	sc := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Provisioner: provisioner.DriverName,
	}

	if isDefault {
		sc.Annotations = map[string]string{
			provisioner.DefaultStorageClassKey: "true",
		}
	}

	return sc
}
