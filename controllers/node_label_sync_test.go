package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	nodelabel "github.com/storageos/api-manager/controllers/node-label"
	"github.com/storageos/api-manager/internal/pkg/annotation"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	nodeLabelSyncTestWorkers       = 1
	defaultNodeLabelResyncInterval = time.Hour // Don't let resync run by default.
)

// SetupNodeLabelSyncTest will set up a testing environment.  It must be
// called from each test.
func SetupNodeLabelSyncTest(ctx context.Context, isStorageOS bool, createLabels map[string]string, resyncInterval time.Duration) *corev1.Node {
	node := &corev1.Node{}

	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(ctx)

		*node = corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "testnode-" + randStringRunes(5),
				Labels: createLabels,
			},
		}

		if isStorageOS {
			driverMap, err := json.Marshal(map[string]string{
				annotation.DriverName: uuid.New().String(),
			})
			Expect(err).NotTo(HaveOccurred(), "failed to marshal csi driver annotation")
			node.Annotations = map[string]string{
				annotation.DriverAnnotationKey: string(driverMap),
			}
		}

		err := k8sClient.Create(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node")

		api = storageos.NewMockClient()
		err = api.AddNode(node.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		controller := nodelabel.NewReconciler(api, mgr.GetClient(), resyncInterval)
		err = controller.SetupWithManager(mgr, nodeLabelSyncTestWorkers)
		Expect(err).NotTo(HaveOccurred(), "failed to setup controller")

		go func() {
			err := mgr.Start(ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to start manager")
		}()

		// Wait for manager to be ready.
		time.Sleep(managerWaitDuration)
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test node")
		cancel()
	})

	return node
}

var _ = Describe("Node Label controller", func() {
	// Define utility constants for object names and testing timeouts/durations
	// and intervals.
	const (
		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	var unreservedLabels = map[string]string{
		"foo": "bar",
		"baz": "boo",
	}
	var reservedLabels = map[string]string{
		storageos.ReservedLabelComputeOnly: "true",
	}
	var mixedLabels = map[string]string{
		"foo":                              "bar",
		"baz":                              "boo",
		storageos.ReservedLabelComputeOnly: "true",
	}

	ctx := context.Background()

	Context("When adding unreserved labels", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should sync labels to StorageOS Node", func() {
			By("By adding unreserved labels to k8s Node")
			node.SetLabels(unreservedLabels)
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
			}, timeout, interval).Should(Equal(unreservedLabels))
		})
	})

	Context("When adding reserved labels", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should sync labels to StorageOS Node", func() {
			By("By adding reserved labels to k8s Node")
			node.SetLabels(reservedLabels)
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
			}, timeout, interval).Should(Equal(reservedLabels))
		})
	})

	Context("When adding mixed labels", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should sync labels to StorageOS Node", func() {
			By("By adding mixed labels to k8s Node")
			node.SetLabels(mixedLabels)
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
			}, timeout, interval).Should(Equal(mixedLabels))
		})
	})

	Context("When adding unrecognised reserved labels", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should only sync recognised labels to StorageOS Node", func() {
			By("By adding unrecognised reserved labels to k8s Node")
			labels := map[string]string{}
			for k, v := range unreservedLabels {
				labels[k] = v
			}
			labels[storageos.ReservedLabelPrefix+"unrecognised"] = "true"
			node.SetLabels(labels)
			Eventually(func() error {
				return k8sClient.Update(ctx, node)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Node labels to match other labels")
			Eventually(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, timeout, interval).Should(Equal(unreservedLabels))
		})
	})

	Context("When adding computeonly label", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should sync labels to StorageOS Node", func() {
			By("By adding computeonly label to k8s Node")
			labels := map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			}
			node.SetLabels(labels)
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
			}, timeout, interval).Should(Equal(labels))
		})
	})

	Context("When adding and removing mixed labels", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should sync labels to StorageOS Node", func() {
			By("By adding labels to k8s Node")
			node.SetLabels(mixedLabels)
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
			}, timeout, interval).Should(Equal(mixedLabels))

			By("By removing labels from k8s Node")
			node.SetLabels(map[string]string{})
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
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When adding computeonly label and the StorageOS API returns an error", func() {
		node := SetupNodeLabelSyncTest(ctx, true, nil, defaultNodeLabelResyncInterval)
		It("Should not sync labels to StorageOS Node", func() {
			By("Setting API to return error")
			api.EnsureNodeLabelsErr = errors.New("fake error")

			By("By adding computeonly label to k8s Node")
			labels := map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			}
			node.SetLabels(labels)
			Eventually(func() error {
				return k8sClient.Update(ctx, node)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Node labels to be empty")
			Eventually(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, timeout, interval).Should(BeEmpty())
		})
	})

	Context("When adding labels on a k8s Node without the StorageOS driver registration", func() {
		node := SetupNodeLabelSyncTest(ctx, false, nil, defaultNodeLabelResyncInterval)
		It("Should not sync labels to StorageOS Node", func() {
			By("By adding labels to k8s Node")
			node.SetLabels(unreservedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, node)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Node labels not to match")
			Consistently(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, duration, interval).ShouldNot(Equal(unreservedLabels))
		})
	})

	Context("When adding labels a k8s Node with a malformed StorageOS driver registration", func() {
		node := SetupNodeLabelSyncTest(ctx, false, nil, defaultNodeLabelResyncInterval)
		It("Should not sync labels to StorageOS Node", func() {
			By("By setting an invalid annotation")
			node.Annotations = map[string]string{
				annotation.DriverAnnotationKey: "{\"csi.storageos.com\":}",
			}
			Expect(k8sClient.Update(ctx, node, &client.UpdateOptions{})).Should(Succeed())

			By("By adding label to k8s Node")
			node.SetLabels(unreservedLabels)
			Eventually(func() error {
				return k8sClient.Update(ctx, node)
			}, timeout, interval).Should(Succeed())

			By("Expecting StorageOS Node labels not to match")
			Consistently(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, duration, interval).ShouldNot(Equal(unreservedLabels))
		})
	})

	Context("When starting after a k8s Node with labels has been created", func() {
		node := SetupNodeLabelSyncTest(ctx, true, mixedLabels, 1*time.Second)
		It("The resync should update the StorageOS Node", func() {
			By("Expecting StorageOS Node labels to match")
			Eventually(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, timeout, interval).Should(Equal(mixedLabels))
		})
	})

})
