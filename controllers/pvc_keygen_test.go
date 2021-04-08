package controllers

import (
	"context"
	"time"

	"github.com/darkowlzz/operator-toolkit/webhook/cert"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	pvcmutator "github.com/storageos/api-manager/controllers/pvc-mutator"
	"github.com/storageos/api-manager/controllers/pvc-mutator/encryption"
)

// SetupPVCKeygenTest will set up a testing environment.  It must be called
// from each test.
func SetupPVCKeygenTest(ctx context.Context) {
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

		decoder, err := admission.NewDecoder(mgr.GetScheme())
		Expect(err).NotTo(HaveOccurred(), "failed to create decoder")

		pvcMutator := pvcmutator.NewController(mgr.GetClient(), decoder, []pvcmutator.Mutator{
			encryption.NewEncryptionKeySetter(mgr.GetClient()),
		})

		mgr.GetWebhookServer().Register(webhookMutatePVCsPath, &webhook.Admission{Handler: pvcMutator})
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
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
		volumeMode := v1.PersistentVolumeFilesystem
		return corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "pvc-" + randStringRunes(5),
				Namespace:   "default",
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{v1.PersistentVolumeAccessMode("ReadWriteOnce")},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				VolumeMode: &volumeMode,
			},
		}
	}

	ctx := context.Background()

	labelsEnabled := map[string]string{
		encryption.EncryptionEnabledLabel: "true",
	}

	Context("When the PVC Mutator has no mutators", func() {
		SetupPVCKeygenTest(ctx)
		It("The pvc should be created", func() {
			pvc := genPVC(map[string]string{}, nil)

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting the PVC to be unchanged")
			Consistently(func() corev1.PersistentVolumeClaim {
				got := corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &got)
				if err != nil {
					return corev1.PersistentVolumeClaim{}
				}
				return got
			}, timeout, interval).Should(Equal(pvc))
		})
	})

	Context("When the PVC has encryption enabled", func() {
		SetupPVCKeygenTest(ctx)
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
					return ""
				}
				secretName = mutatedPVC.Annotations[encryption.EncryptionSecretNameAnnotationKey]
				return secretName
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}
				secretNamespace = mutatedPVC.Annotations[encryption.EncryptionSecretNamespaceAnnotationKey]
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
		})
	})

	Context("When the PVC has key name annotation set", func() {
		SetupPVCKeygenTest(ctx)
		It("The pvc should be created", func() {
			pvc := genPVC(labelsEnabled, map[string]string{
				encryption.EncryptionSecretNameAnnotationKey: "my-key",
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
					return ""
				}
				secretName = mutatedPVC.Annotations[encryption.EncryptionSecretNameAnnotationKey]
				return secretName
			}, timeout, interval).Should(Equal("my-key"))

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}
				secretNamespace = mutatedPVC.Annotations[encryption.EncryptionSecretNamespaceAnnotationKey]
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
		})
	})

	Context("When the PVC has key namespace annotation set", func() {
		SetupPVCKeygenTest(ctx)
		It("The pvc should be created", func() {
			pvc := genPVC(labelsEnabled, map[string]string{
				encryption.EncryptionSecretNamespaceAnnotationKey: "default",
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
					return ""
				}
				secretName = mutatedPVC.Annotations[encryption.EncryptionSecretNameAnnotationKey]
				return secretName
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Expecting secret namespace annotation to be set")
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return ""
				}
				secretNamespace = mutatedPVC.Annotations[encryption.EncryptionSecretNamespaceAnnotationKey]
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
		})
	})

	Context("When the PVC has key namespace annotation set to a namespace different than the PVC", func() {
		SetupPVCKeygenTest(ctx)
		It("The pvc should not be created", func() {
			pvc := genPVC(labelsEnabled, map[string]string{
				encryption.EncryptionSecretNamespaceAnnotationKey: "another-users-namespace",
			})

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).ShouldNot(Succeed())
		})
	})

})
