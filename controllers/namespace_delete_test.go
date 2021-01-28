package controllers

import (
	"context"
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	nsdelete "github.com/storageos/api-manager/controllers/namespace-delete"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// SetupNamespaceDeleteTest will set up a testing environment.  It must be
// called from each test.
func SetupNamespaceDeleteTest(ctx context.Context, createK8sNamespace bool, gcEnabled bool) *corev1.Namespace {
	ns := &corev1.Namespace{}

	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "testns-" + randStringRunes(5)},
		}

		if createK8sNamespace {
			err := k8sClient.Create(ctx, ns)
			Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")
		}

		api = storageos.NewMockClient()
		err := api.AddNamespace(ns.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		gcInterval := defaultSyncInterval
		if gcEnabled {
			gcInterval = time.Hour
		}

		controller := nsdelete.NewReconciler(api, mgr.GetClient(), defaultSyncDelay, gcInterval)
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
		// Don't delete the namespace, the test should have.
		// Stop the manager.
		cancel()
	})

	return ns
}

var _ = Describe("Namespace Delete controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	ctx := context.Background()

	Context("When deleting a k8s Namespace", func() {
		ns := SetupNamespaceDeleteTest(ctx, true, false)
		It("Should delete the StorageOS Namespace", func() {
			// Skip test if running in envtest.  envtest doesn't handle namespace
			// deletion in the same way as other objects, so the delete event is never
			// sent: https://github.com/kubernetes-sigs/controller-runtime/issues/880
			if val, ok := os.LookupEnv("USE_EXISTING_CLUSTER"); !ok || val == "false" {
				Skip("Namespace delete events not seen in envtest")
			}

			By("By deleting the k8s Namespace")
			Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())

			By("Expecting StorageOS Namespace to be deleted")
			Eventually(func() bool {
				return api.NamespaceExists(ns.Name)
			}, timeout, interval).Should(BeFalse())
		})
	})

	Context("When deleting a k8s Namespace and the StorageOS Namespace has already been deleted", func() {
		ns := SetupNamespaceDeleteTest(ctx, true, false)
		It("Should not fail", func() {
			// Skip test if running in envtest.  envtest doesn't handle namespace
			// deletion in the same way as other objects, so the delete event is never
			// sent: https://github.com/kubernetes-sigs/controller-runtime/issues/880
			if val, ok := os.LookupEnv("USE_EXISTING_CLUSTER"); !ok || val == "false" {
				Skip("Namespace delete events not seen in envtest")
			}

			By("By causing the StorageOS Namespace delete to fail with a 404")
			api.DeleteNamespaceErr = storageos.ErrNamespaceNotFound

			By("By deleting the k8s Namespace")
			Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())

			By("Expecting StorageOS Delete Namespace to be called only once")
			Eventually(func() int {
				return api.DeleteNamespaceCallCount[ns.Name]
			}, timeout, interval).Should(Equal(1))
			Consistently(func() int {
				return api.DeleteNamespaceCallCount[ns.Name]
			}, duration, interval).Should(Equal(1))
		})
	})

	Context("When deleting a k8s Namespace that still has volumes", func() {
		ns := SetupNamespaceDeleteTest(ctx, true, false)
		It("Should not delete the StorageOS Namespace", func() {
			// Skip test if running in envtest.  envtest doesn't handle namespace
			// deletion in the same way as other objects, so the delete event is never
			// sent: https://github.com/kubernetes-sigs/controller-runtime/issues/880
			if val, ok := os.LookupEnv("USE_EXISTING_CLUSTER"); !ok || val == "false" {
				Skip("Namespace delete events not seen in envtest")
			}

			By("By causing the StorageOS Namespace delete to fail")
			api.DeleteNamespaceErr = errors.New("delete failed")

			By("By deleting the k8s Namespace")
			Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())

			By("Expecting StorageOS Namespace to not be deleted")
			Consistently(func() bool {
				return api.NamespaceExists(ns.Name)
			}, duration, interval).Should(BeTrue())

			By("Expecting StorageOS Delete Namespace to be called multiple times")
			Eventually(func() int {
				return api.DeleteNamespaceCallCount[ns.Name]
			}, timeout, interval).Should(BeNumerically(">", 1))
		})
	})

	Context("When starting after a k8s Namespace has been deleted but is still in StorageOS", func() {
		ns := SetupNamespaceDeleteTest(ctx, false, true)
		It("The garbage collector should delete the StorageOS Namespace", func() {
			By("Expecting StorageOS Namespace to be deleted")
			Eventually(func() bool {
				return api.NamespaceExists(ns.Name)
			}, timeout, interval).Should(BeFalse())
		})
	})

})
