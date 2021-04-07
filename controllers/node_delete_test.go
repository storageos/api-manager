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
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodedelete "github.com/storageos/api-manager/controllers/node-delete"
	"github.com/storageos/api-manager/internal/pkg/provisioner"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// SetupNodeDeleteTest will set up a testing environment.  It must be called
// from each test.
func SetupNodeDeleteTest(ctx context.Context, createK8sNode bool, isStorageOS bool, gcEnabled bool) client.ObjectKey {
	var key = client.ObjectKey{Name: "testnode-" + randStringRunes(5)}
	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		if createK8sNode {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name},
			}
			if isStorageOS {
				k, v := getCSIAnnotation()
				node.Annotations = map[string]string{
					k: v,
				}
			}
			err := k8sClient.Create(ctx, node)
			Expect(err).NotTo(HaveOccurred(), "failed to create test node")
		}

		api = storageos.NewMockClient()
		err := api.AddNode(storageos.MockObject{Name: key.Name})
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{MetricsBindAddress: "0"})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		gcInterval := defaultSyncInterval
		if gcEnabled {
			gcInterval = time.Hour
		}

		controller := nodedelete.NewReconciler(api, mgr.GetClient(), defaultSyncDelay, gcInterval)
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
		// Don't delete the node, the test should have.
		// Stop the manager.
		cancel()
	})

	return key
}

var _ = Describe("Node Delete controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	ctx := context.Background()

	Context("When deleting a k8s Node with the StorageOS driver registered", func() {
		key := SetupNodeDeleteTest(ctx, true, true, false)
		It("Should delete the StorageOS Node", func() {
			By("Confirming Node exists in k8s")
			var node corev1.Node
			Expect(k8sClient.Get(ctx, key, &node)).Should(Succeed())

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, &node)).Should(Succeed())

			By("Expecting StorageOS Node to be deleted")
			Eventually(func() bool {
				return api.NodeExists(key)
			}, timeout, interval).Should(BeFalse())
		})
	})

	Context("When deleting a k8s Node and the StorageOS node has already been deleted", func() {
		key := SetupNodeDeleteTest(ctx, true, true, false)
		It("Should not fail", func() {
			By("By causing the StorageOS Node delete to fail with a 404")
			api.DeleteNodeErr = storageos.ErrNodeNotFound

			By("Confirming Node exists in k8s")
			var node corev1.Node
			Expect(k8sClient.Get(ctx, key, &node)).Should(Succeed())

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, &node)).Should(Succeed())

			By("Expecting StorageOS Delete Node to be called only once")
			Eventually(func() int {
				return api.DeleteNodeCallCount[key]
			}, timeout, interval).Should(Equal(1))
			Consistently(func() int {
				return api.DeleteNodeCallCount[key]
			}, duration, interval).Should(Equal(1))
		})
	})

	Context("When deleting a k8s Node without the StorageOS driver registration", func() {
		key := SetupNodeDeleteTest(ctx, true, false, false)
		It("Should not delete the StorageOS Node", func() {

			By("Confirming Node exists in k8s")
			var node corev1.Node
			Expect(k8sClient.Get(ctx, key, &node)).Should(Succeed())

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, &node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(key)
			}, duration, interval).Should(BeTrue())
		})
	})

	Context("When deleting a k8s Node with a malformed StorageOS driver registration", func() {
		key := SetupNodeDeleteTest(ctx, true, false, false)
		It("Should not delete the StorageOS Node", func() {
			// Skip test if not running in envtest.  k8s will disallow the
			// malformed annotation.
			if val, ok := os.LookupEnv("USE_EXISTING_CLUSTER"); ok && val == "true" {
				Skip("k8s will reject malformed annotation")
			}

			By("Confirming Node exists in k8s")
			var node corev1.Node
			Expect(k8sClient.Get(ctx, key, &node)).Should(Succeed())

			By("By setting an invalid annotation")
			node.Annotations = map[string]string{
				provisioner.NodeDriverAnnotationKey: "{\"csi.storageos.com\":}",
			}
			Expect(k8sClient.Update(ctx, &node, &client.UpdateOptions{})).Should(Succeed())

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, &node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(key)
			}, duration, interval).Should(BeTrue())
		})
	})

	Context("When deleting a k8s Node that still has volumes", func() {
		key := SetupNodeDeleteTest(ctx, true, true, false)
		It("Should not delete the StorageOS Node", func() {
			By("By causing the StorageOS Node delete to fail")
			api.DeleteNodeErr = errors.New("delete failed")

			By("Confirming Node exists in k8s")
			var node corev1.Node
			Expect(k8sClient.Get(ctx, key, &node)).Should(Succeed())

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, &node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(key)
			}, duration, interval).Should(BeTrue())

			By("Expecting StorageOS Delete Node to be called multiple times")
			Eventually(func() int {
				return api.DeleteNodeCallCount[key]
			}, timeout, interval).Should(BeNumerically(">", 1))
		})
	})

	Context("When starting after a k8s Node has been deleted but is still in StorageOS", func() {
		key := SetupNodeDeleteTest(ctx, false, true, true)
		It("The garbage collector should delete the StorageOS Node", func() {
			By("Expecting StorageOS Node to be deleted")
			Eventually(func() bool {
				return api.NodeExists(key)
			}, timeout, interval).Should(BeFalse())
		})
	})

})
