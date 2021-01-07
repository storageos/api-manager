package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	nodedelete "github.com/storageos/api-manager/controllers/node-delete"
	"github.com/storageos/api-manager/internal/pkg/annotations"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	nodeDeleteTestWorkers = 1
)

// SetupNodeDeleteTest will set up a testing environment.  It must be called
// from each test.
func SetupNodeDeleteTest(ctx context.Context, isStorageOS bool) *corev1.Node {
	node := &corev1.Node{}

	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		*node = corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "testnode-" + randStringRunes(5)},
		}

		if isStorageOS {
			driverMap, err := json.Marshal(map[string]string{
				annotations.DriverName: uuid.New().String(),
			})
			Expect(err).NotTo(HaveOccurred(), "failed to mars")
			node.Annotations = map[string]string{
				annotations.DriverAnnotationKey: string(driverMap),
			}
		}

		err := k8sClient.Create(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node")

		api = storageos.NewMockClient()
		err = api.AddNode(node.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		controller := nodedelete.NewReconciler(api, mgr.GetClient())
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
		node := SetupNodeDeleteTest(ctx, true)
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
		node := SetupNodeDeleteTest(ctx, true)
		It("Should not fail", func() {
			Expect(api.DeleteNode(node.Name)).Should(Succeed())
			api.DeleteNodeErr = storageos.ErrNodeNotFound

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Node to be deleted")
			Eventually(func() bool {
				return api.NodeExists(node.Name)
			}, timeout, interval).Should(BeFalse())
		})
	})

	Context("When deleting a k8s Node without the StorageOS driver registration", func() {
		node := SetupNodeDeleteTest(ctx, false)
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
		node := SetupNodeDeleteTest(ctx, false)
		It("Should not delete the StorageOS Node", func() {
			By("By setting an invalid annotation")
			node.Annotations = map[string]string{
				annotations.DriverAnnotationKey: "{\"csi.storageos.com\":}",
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
		node := SetupNodeDeleteTest(ctx, true)
		It("Should not delete the StorageOS Node", func() {
			By("By causing the StorageOS Node delete to fail")
			api.DeleteNodeErr = errors.New("delete failed")

			By("By deleting the k8s Node")
			Expect(k8sClient.Delete(ctx, node)).Should(Succeed())

			By("Expecting StorageOS Node to not be deleted")
			Consistently(func() bool {
				return api.NodeExists(node.Name)
			}, duration, interval).Should(BeTrue())
		})
	})

})
