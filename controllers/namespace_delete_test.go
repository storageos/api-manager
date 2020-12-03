package controllers

import (
	"context"
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	nsdelete "github.com/storageos/api-manager/controllers/namespace-delete"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	nsDeleteTestWorkers = 1
)

// SetupNamespaceDeleteTest will set up a testing environment.  It must be
// called from each test.
func SetupNamespaceDeleteTest(ctx context.Context) *corev1.Namespace {
	var stopCh chan struct{}
	ns := &corev1.Namespace{}

	BeforeEach(func() {
		stopCh = make(chan struct{})
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "testns-" + randStringRunes(5)},
		}

		err := k8sClient.Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

		api = storageos.NewMockClient()
		err = api.AddNamespace(ns.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		controller := nsdelete.NewReconciler(api, mgr.GetClient())
		err = controller.SetupWithManager(mgr, nsDeleteTestWorkers)
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(stopCh)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		close(stopCh)
		// Don't delete the namespace, the test should have.
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
		ns := SetupNamespaceDeleteTest(ctx)
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
		ns := SetupNamespaceDeleteTest(ctx)
		It("Should not fail", func() {
			// Skip test if running in envtest.  envtest doesn't handle namespace
			// deletion in the same way as other objects, so the delete event is never
			// sent: https://github.com/kubernetes-sigs/controller-runtime/issues/880
			if val, ok := os.LookupEnv("USE_EXISTING_CLUSTER"); !ok || val == "false" {
				Skip("Namespace delete events not seen in envtest")
			}
			Expect(api.DeleteNamespace(ns.Name)).Should(Succeed())
			api.DeleteNamespaceErr = storageos.ErrNamespaceNotFound

			By("By deleting the k8s Namespace")
			Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())

			By("Expecting StorageOS Namespace to be deleted")
			Eventually(func() bool {
				return api.NamespaceExists(ns.Name)
			}, timeout, interval).Should(BeFalse())
		})
	})

	Context("When deleting a k8s Namespace that still has volumes", func() {
		ns := SetupNamespaceDeleteTest(ctx)
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
		})
	})

})
