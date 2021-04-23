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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	podmutator "github.com/storageos/api-manager/controllers/pod-mutator"
)

// SetupPodMutatorTest will set up a testing environment.  It must be called
// from each test.
func SetupPodMutatorTest(ctx context.Context, mutators []podmutator.Mutator) {
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

		podMutator := podmutator.NewController(compositeClient, decoder, mutators)
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
		cancel()
	})
}

var _ = Describe("Pod Mutator controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 1
		duration = time.Second * 1
		interval = time.Millisecond * 250
	)

	genPod := func() corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-" + randStringRunes(5),
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{},
				Containers: []corev1.Container{
					{
						Name:  "test-app",
						Image: "nginx",
					},
				},
			},
		}
	}

	ctx := context.Background()

	Context("When the Pod Mutator has no mutators", func() {
		SetupPodMutatorTest(ctx, nil)
		It("The pod should be created", func() {
			pod := genPod()

			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod to be unchanged")
			Consistently(func() corev1.Pod {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return corev1.Pod{}
				}
				return got
			}, timeout, interval).Should(Equal(pod))
		})
	})

	Context("When the Pod Mutator has one mutator", func() {
		SetupPodMutatorTest(ctx, []podmutator.Mutator{testMutator{key: "foo", value: "bar"}})
		It("The pod should be created", func() {
			pod := genPod()

			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod to be changed")
			Eventually(func() map[string]string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return nil
				}
				return got.GetAnnotations()
			}, timeout, interval).Should(Equal(map[string]string{
				"foo": "bar",
			}))
		})
	})

	Context("When the Pod Mutator has two mutators", func() {
		SetupPodMutatorTest(ctx, []podmutator.Mutator{
			testMutator{key: "foo", value: "bar"},
			testMutator{key: "baz", value: "zab"},
		})
		It("The pod should be created", func() {
			pod := genPod()

			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
			}()

			By("Expecting the Pod to be changed")
			Eventually(func() map[string]string {
				got := corev1.Pod{}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&pod), &got)
				if err != nil {
					return nil
				}
				return got.GetAnnotations()
			}, timeout, interval).Should(Equal(map[string]string{
				"foo": "bar",
				"baz": "zab",
			}))
		})
	})

	Context("When the Pod Mutator has one mutator that errors", func() {
		SetupPodMutatorTest(ctx, []podmutator.Mutator{testMutator{error: true}})
		It("The pod should not be created", func() {
			pod := genPod()

			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).ShouldNot(Succeed())
		})
	})

	Context("When the Pod Mutator has two mutators and one errors", func() {
		SetupPodMutatorTest(ctx, []podmutator.Mutator{
			testMutator{key: "foo", value: "bar"},
			testMutator{error: true}},
		)
		It("The pod should not be created", func() {
			pod := genPod()

			By("Creating the Pod")
			Expect(k8sClient.Create(ctx, &pod)).ShouldNot(Succeed())
		})
	})

})
