package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodedelete "github.com/storageos/api-manager/controllers/node-delete"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

const (
	nodeDeleteTestWorkers       = 1
	defaultNodeDeleteGCInterval = time.Hour // Don't let gc run by default.
)

// SetupNodeDeleteTest will set up a testing environment.  It must be called
// from each test.
func SetupNodeDeleteTest(ctx context.Context, createK8sNode bool, isStorageOS bool, gcInterval time.Duration) *corev1.Node {
	node := &corev1.Node{}

	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		*node = corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "testnode-" + randStringRunes(5)},
		}

		if isStorageOS {
			driverMap, err := json.Marshal(map[string]string{
				nodedelete.DriverName: uuid.New().String(),
			})
			Expect(err).NotTo(HaveOccurred(), "failed to mars")
			node.Annotations = map[string]string{
				nodedelete.DriverAnnotationKey: string(driverMap),
			}
		}

		if createK8sNode {
			err := k8sClient.Create(ctx, node)
			Expect(err).NotTo(HaveOccurred(), "failed to create test node")
		}

		api = storageos.NewMockClient()
		err := api.AddNode(node.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		controller := nodedelete.NewReconciler(api, mgr.GetClient(), gcInterval)
		err = controller.SetupWithManager(mgr, nodeDeleteTestWorkers)
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

	return node
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
		node := SetupNodeDeleteTest(ctx, true, true, defaultNodeDeleteGCInterval)
		It("Should delete the StorageOS Node", func() {
			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Node to be deleted")
			Eventually(func() bool {
				return api.NodeExists(node.Name)
			}, timeout, interval).Should(BeFalse())
		})
	})

	Context("When deleting a k8s Node and the StorageOS node has already been deleted", func() {
		node := SetupNodeDeleteTest(ctx, true, true, defaultNodeDeleteGCInterval)
		It("Should not fail", func() {
			By("By causing the StorageOS Node delete to fail with a 404")
			api.DeleteNodeErr = storageos.ErrNodeNotFound

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Delete Node to be called only once")
			Eventually(func() int {
				return api.DeleteNodeCallCount[node.Name]
			}, timeout, interval).Should(Equal(1))
			Consistently(func() int {
				return api.DeleteNodeCallCount[node.Name]
			}, duration, interval).Should(Equal(1))
		})
	})

	Context("When deleting a k8s Node without the StorageOS driver registration", func() {
		node := SetupNodeDeleteTest(ctx, true, false, defaultNodeDeleteGCInterval)
		It("Should not delete the StorageOS Node", func() {
			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(node.Name)
			}, duration, interval).Should(BeTrue())
		})
	})

	Context("When deleting a k8s Node with a malformed StorageOS driver registration", func() {
		node := SetupNodeDeleteTest(ctx, true, false, defaultNodeDeleteGCInterval)
		It("Should not delete the StorageOS Node", func() {
			// Skip test if not running in envtest.  k8s will disallow the
			// malformed annotation.
			if val, ok := os.LookupEnv("USE_EXISTING_CLUSTER"); ok && val == "true" {
				Skip("k8s will reject malformed annotation")
			}

			By("By setting an invalid annotation")
			node.Annotations = map[string]string{
				nodedelete.DriverAnnotationKey: "{\"csi.storageos.com\":}",
			}
			Expect(k8sClient.Update(ctx, node, &client.UpdateOptions{})).Should(Succeed())

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(node.Name)
			}, duration, interval).Should(BeTrue())
		})
	})

	Context("When deleting a k8s Node that still has volumes", func() {
		node := SetupNodeDeleteTest(ctx, true, true, defaultNodeDeleteGCInterval)
		It("Should not delete the StorageOS Node", func() {
			By("By causing the StorageOS Node delete to fail")
			api.DeleteNodeErr = errors.New("delete failed")

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(node.Name)
			}, duration, interval).Should(BeTrue())

			By("Expecting StorageOS Delete Node to be called multiple times")
			Eventually(func() int {
				return api.DeleteNodeCallCount[node.Name]
			}, timeout, interval).Should(BeNumerically(">", 1))
		})
	})

	Context("When starting after a k8s Node has been deleted but is still in StorageOS", func() {
		node := SetupNodeDeleteTest(ctx, false, true, 1*time.Second)
		It("The garbage collector should delete the StorageOS Node", func() {
			By("Expecting StorageOS Node to be deleted")
			Eventually(func() bool {
				return api.NodeExists(node.Name)
			}, timeout, interval).Should(BeFalse())
		})
	})

})
