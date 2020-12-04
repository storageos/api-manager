package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	nodelabel "github.com/storageos/api-manager/controllers/node-label"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupNodeLabelTest will set up a testing environment.  It must be
// called from each test.
func SetupNodeLabelTest(ctx context.Context) *corev1.Node {
	var stopCh chan struct{}
	node := &corev1.Node{}

	BeforeEach(func() {
		stopCh = make(chan struct{})
		*node = corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "testnode-" + randStringRunes(5)},
		}

		err := k8sClient.Create(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node")

		api = storageos.NewMockClient()
		err = api.AddNode(node.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		controller := nodelabel.NewReconciler(api, mgr.GetClient(), mgr.GetEventRecorderFor("node-label"))
		err = controller.SetupWithManager(mgr)
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
		err := k8sClient.Delete(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test node")
	})

	return node
}

var _ = Describe("Node Label controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var nodeLabels = map[string]string{
		"foo": "bar",
		"baz": "boo",
	}

	ctx := context.Background()

	Context("When updating k8s Node labels", func() {
		node := SetupNodeLabelTest(ctx)
		It("Should sync labels to StorageOS Node", func() {
			By("Expecting k8s Node to exist")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: node.Name}, node)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Node labels to be empty")
			Eventually(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, timeout, interval).Should(BeEmpty())

			By("By adding label to k8s Node")
			node.SetLabels(nodeLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, node)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Node labels to match")
			Eventually(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, timeout, interval).Should(Equal(nodeLabels))
		})
	})

})
