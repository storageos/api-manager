package controllers

import (
	"context"
	"time"

	"github.com/darkowlzz/operator-toolkit/webhook/cert"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	pvcmutator "github.com/storageos/api-manager/controllers/pvc-mutator"
	sclabel "github.com/storageos/api-manager/controllers/pvc-mutator/sc-label"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

var storageClassLabels = map[string]string{storageos.ReservedLabelPrefix + "param": "value"}

// SetupPVCSCLabelTest will set up a testing environment.  It must be called
// from each test.
func SetupPVCSCLabelTest(ctx context.Context, addMutator bool) {
	var cancel func()

	sc := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				defaultStorageClassKey: "true",
			},
		},
		Provisioner: provisioner.DriverName,
		Parameters:  storageClassLabels,
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

		if addMutator {
			pvcMutator := pvcmutator.NewController(mgr.GetClient(), decoder, []pvcmutator.Mutator{
				sclabel.NewLabelSetter(mgr.GetClient()),
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

var _ = Describe("PVC StorageClass label sync controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 1
		duration = time.Second * 1
		interval = time.Millisecond * 250
	)

	genPVC := func() corev1.PersistentVolumeClaim {
		volumeMode := corev1.PersistentVolumeFilesystem
		return corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc-" + randStringRunes(5),
				Namespace: "default",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode("ReadWriteOnce")},
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

	Context("When the PVC Mutator has no mutators", func() {
		SetupPVCSCLabelTest(ctx, false)
		It("The pvc should be created", func() {
			pvc := genPVC()

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

	Context("When the PVC Mutator has mutators", func() {
		SetupPVCSCLabelTest(ctx, true)
		It("The pvc should be created", func() {
			pvc := genPVC()

			By("Creating the PVC")
			Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pvc)).Should(Succeed())
			}()

			By("Expecting params have configured as labels")
			Eventually(func() map[string]string {
				var mutatedPVC corev1.PersistentVolumeClaim

				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pvc), &mutatedPVC)
				if err != nil {
					return nil
				}

				return mutatedPVC.Labels
			}, timeout, interval).Should(Equal(storageClassLabels))
		})
	})
})
