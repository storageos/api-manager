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
	"github.com/storageos/api-manager/controllers/pvc-mutator/encryption"
	"github.com/storageos/api-manager/internal/pkg/labels"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// SetupPVCKeygenTest will set up a testing environment.  It must be called
// from each test.
func SetupPVCKeygenTest(ctx context.Context, addMutator bool, isStorageOS bool, isStorageClassEncrypted bool) {
	var cancel func()

	sc := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				provisioner.DefaultStorageClassKey: "true",
			},
		},
		Provisioner: "foreign",
	}
	if isStorageOS {
		sc.Provisioner = provisioner.DriverName
	}
	if isStorageClassEncrypted {
		sc.Parameters = map[string]string{
			storageos.ReservedLabelEncryption: "true",
		}
	}

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

		decoder, err := admission.NewDecoder(mgr.GetScheme())
		Expect(err).NotTo(HaveOccurred(), "failed to create decoder")

		compositeClient, err := cclient.NewClientFromManager(mgr, cclient.Options{})
		if err != nil {
			Expect(err).NotTo(HaveOccurred(), "failed to create composite client")
			return
		}

		if addMutator {
			pvcMutator := pvcmutator.NewController(compositeClient, decoder, []pvcmutator.Mutator{
				encryption.NewKeySetter(compositeClient, labels.Default()),
			})

			mgr.GetWebhookServer().Register(webhookMutatePVCsPath, &webhook.Admission{Handler: pvcMutator})
			Expect(err).NotTo(HaveOccurred(), "failed to setup controller")
		}

		Expect(k8sClient.Create(ctx, &sc)).Should(Succeed())

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &sc)).Should(Succeed())

		cancel()
	})
}

var _ = Describe("PVC Keygen controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 1
		duration = time.Second * 1
		interval = time.Millisecond * 250
	)

	genPVC := func(labels map[string]string, annotations map[string]string) corev1.PersistentVolumeClaim {
		pvc := genPVC()

		pvc.Labels = labels
		pvc.Annotations = annotations
		vm := corev1.PersistentVolumeFilesystem
		pvc.Spec.VolumeMode = &vm

		return pvc
	}

	ctx := context.Background()

	labelsEnabled := map[string]string{
		storageos.ReservedLabelEncryption: "true",
	}

	Context("When the PVC Mutator has no mutators", func() {
		SetupPVCKeygenTest(ctx, false, true, false)
		It("The pvc should be created", func() {
			pvc := genPVC(map[string]string{}, nil)

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC not to have any annotations")
			// The create operation mutates the PVC, so the testcase has
			// reference only to the final version. Only annotations could be tested.
			Consistently(func() map[string]string {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return nil
				}
				return got.Annotations
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When the PVC has encryption enabled but is not StorageOS", func() {
		SetupPVCKeygenTest(ctx, true, false, false)
		It("The pvc should be created", func() {
			pvc := genPVC(labelsEnabled, nil)

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC not to have any annotations")
			// The create operation mutates the PVC, so the testcase has
			// reference only to the final version. Only annotations could be tested.
			Consistently(func() map[string]string {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return nil
				}
				return got.Annotations
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When the PVC has encryption disabled", func() {
		SetupPVCKeygenTest(ctx, true, false, false)
		It("The pvc should be created", func() {
			labelsDisabled := map[string]string{
				storageos.ReservedLabelEncryption: "false",
			}
			pvc := genPVC(labelsDisabled, nil)

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC not to have any annotations")
			// The create operation mutates the PVC, so the testcase has
			// reference only to the final version. Only annotations could be tested.
			Consistently(func() map[string]string {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return nil
				}
				return got.Annotations
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When the PVC has encryption enabled", func() {
		SetupPVCKeygenTest(ctx, true, true, false)
		It("The pvc should be created", func() {
			pvc := genPVC(labelsEnabled, nil)

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			// Scoped here so we can access in multiple tests.
			var (
				mutatedPVC      corev1.PersistentVolumeClaim
				secretName      string
				secretNamespace string
			)

			By("Expecting secret name annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretName = mutatedPVC.Annotations[encryption.SecretNameAnnotationKey]
				return secretName
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretNamespace = mutatedPVC.Annotations[encryption.SecretNamespaceAnnotationKey]
				return secretNamespace
			}, timeout, interval).Should(Equal(pvc.GetNamespace()))

			secretRef := client.ObjectKey{
				Name:      secretName,
				Namespace: secretNamespace,
			}

			By("Expecting secret to exist with 64-bit key set")
			Eventually(func() int {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return 0
				}
				if got.Data == nil {
					return 0
				}
				return len(got.Data["key"])
			}, timeout, interval).Should(Equal(64))

			By("Expecting secret to have correct labels set")
			Eventually(func() map[string]string {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return nil
				}
				return got.GetLabels()
			}, timeout, interval).Should(Equal(map[string]string{
				labels.AppComponent:                 labels.DefaultAppComponent,
				labels.AppManagedBy:                 labels.DefaultAppManagedBy,
				labels.AppName:                      labels.DefaultAppName,
				labels.AppPartOf:                    labels.DefaultAppPartOf,
				encryption.VolumeSecretPVCNameLabel: pvc.Name,
			}))
		})
	})

	Context("When the StorageClass of PVC has encryption enabled", func() {
		SetupPVCKeygenTest(ctx, true, true, true)
		It("The pvc should be created", func() {
			pvc := genPVC(nil, nil)

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			// Scoped here so we can access in multiple tests.
			var (
				mutatedPVC      corev1.PersistentVolumeClaim
				secretName      string
				secretNamespace string
			)

			By("Expecting secret name annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretName = mutatedPVC.Annotations[encryption.SecretNameAnnotationKey]
				return secretName
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretNamespace = mutatedPVC.Annotations[encryption.SecretNamespaceAnnotationKey]
				return secretNamespace
			}, timeout, interval).Should(Equal(pvc.GetNamespace()))

			secretRef := client.ObjectKey{
				Name:      secretName,
				Namespace: secretNamespace,
			}

			By("Expecting secret to exist with 64-bit key set")
			Eventually(func() int {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return 0
				}
				if got.Data == nil {
					return 0
				}
				return len(got.Data["key"])
			}, timeout, interval).Should(Equal(64))

			By("Expecting secret to have correct labels set")
			Eventually(func() map[string]string {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return nil
				}
				return got.GetLabels()
			}, timeout, interval).Should(Equal(map[string]string{
				labels.AppComponent:                 labels.DefaultAppComponent,
				labels.AppManagedBy:                 labels.DefaultAppManagedBy,
				labels.AppName:                      labels.DefaultAppName,
				labels.AppPartOf:                    labels.DefaultAppPartOf,
				encryption.VolumeSecretPVCNameLabel: pvc.Name,
			}))
		})
	})

	Context("When the PVC has key name annotation set", func() {
		SetupPVCKeygenTest(ctx, true, true, false)
		It("The pvc should be created", func() {
			pvc := genPVC(labelsEnabled, map[string]string{
				encryption.SecretNameAnnotationKey: "my-key",
			})

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			// Scoped here so we can access in multiple tests.
			var (
				mutatedPVC      corev1.PersistentVolumeClaim
				secretName      string
				secretNamespace string
			)

			By("Expecting secret name annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretName = mutatedPVC.Annotations[encryption.SecretNameAnnotationKey]
				return secretName
			}, timeout, interval).Should(Equal("my-key"))

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretNamespace = mutatedPVC.Annotations[encryption.SecretNamespaceAnnotationKey]
				return secretNamespace
			}, timeout, interval).Should(Equal(pvc.GetNamespace()))

			secretRef := client.ObjectKey{
				Name:      secretName,
				Namespace: secretNamespace,
			}

			By("Expecting secret to exist with 64-bit key set")
			Eventually(func() int {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return 0
				}
				if got.Data == nil {
					return 0
				}
				return len(got.Data["key"])
			}, timeout, interval).Should(Equal(64))

			By("Expecting secret to have correct labels set")
			Eventually(func() map[string]string {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return nil
				}
				return got.GetLabels()
			}, timeout, interval).Should(Equal(map[string]string{
				labels.AppComponent:                 labels.DefaultAppComponent,
				labels.AppManagedBy:                 labels.DefaultAppManagedBy,
				labels.AppName:                      labels.DefaultAppName,
				labels.AppPartOf:                    labels.DefaultAppPartOf,
				encryption.VolumeSecretPVCNameLabel: pvc.Name,
			}))
		})
	})

	Context("When the PVC has key namespace annotation set", func() {
		SetupPVCKeygenTest(ctx, true, true, false)
		It("The pvc should be created", func() {
			pvc := genPVC(labelsEnabled, map[string]string{
				encryption.SecretNamespaceAnnotationKey: "default",
			})

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			// Scoped here so we can access in multiple tests.
			var (
				mutatedPVC      corev1.PersistentVolumeClaim
				secretName      string
				secretNamespace string
			)

			By("Expecting secret name annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretName = mutatedPVC.Annotations[encryption.SecretNameAnnotationKey]
				return secretName
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return err.Error()
				}
				secretNamespace = mutatedPVC.Annotations[encryption.SecretNamespaceAnnotationKey]
				return secretNamespace
			}, timeout, interval).Should(Equal(pvc.GetNamespace()))

			secretRef := client.ObjectKey{
				Name:      secretName,
				Namespace: secretNamespace,
			}

			By("Expecting secret to exist with 64-bit key set")
			Eventually(func() int {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return 0
				}
				if got.Data == nil {
					return 0
				}
				return len(got.Data["key"])
			}, timeout, interval).Should(Equal(64))

			By("Expecting secret to have correct labels set")
			Eventually(func() map[string]string {
				got := corev1.Secret{}
				err := k8sClient.Get(ctx, secretRef, &got)
				if err != nil {
					return nil
				}
				return got.GetLabels()
			}, timeout, interval).Should(Equal(map[string]string{
				labels.AppComponent:                 labels.DefaultAppComponent,
				labels.AppManagedBy:                 labels.DefaultAppManagedBy,
				labels.AppName:                      labels.DefaultAppName,
				labels.AppPartOf:                    labels.DefaultAppPartOf,
				encryption.VolumeSecretPVCNameLabel: pvc.Name,
			}))
		})
	})

	Context("When the PVC has key namespace annotation set to a namespace different than the PVC", func() {
		SetupPVCKeygenTest(ctx, true, true, false)
		It("The pvc should not be created", func() {
			pvc := genPVC(labelsEnabled, map[string]string{
				encryption.SecretNamespaceAnnotationKey: "another-users-namespace",
			})

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).ShouldNot(Succeed())
		})
	})

})
