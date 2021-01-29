package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodelabel "github.com/storageos/api-manager/controllers/node-label"
	"github.com/storageos/api-manager/internal/pkg/annotation"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

// SetupNodeLabelSyncTest will set up a testing environment.  It must be
// called from each test.
func SetupNodeLabelSyncTest(ctx context.Context, isStorageOS bool, createLabels map[string]string, gcEnabled bool) *corev1.Node {
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
			k, v := getCSIAnnotation()
			node.Annotations = map[string]string{
				k: v,
			}
		}

		err := k8sClient.Create(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node")

		api = storageos.NewMockClient()
		err = api.AddNode(node.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to create test node in storageos")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		gcInterval := defaultSyncInterval
		if gcEnabled {
			gcInterval = time.Hour
		}

		controller := nodelabel.NewReconciler(api, mgr.GetClient(), defaultSyncDelay, gcInterval)
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
		err := k8sClient.Delete(ctx, node)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test node")
		cancel()
	})

	return node
}

// getCSIAnnotation is a helper to return a valid StorageOS CSI Driver annotation.
func getCSIAnnotation() (string, string) {
	driverMap, _ := json.Marshal(map[string]string{
		annotation.DriverName: uuid.New().String(),
	})
	return annotation.DriverAnnotationKey, string(driverMap)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, true, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, false, nil, false)
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
		node := SetupNodeLabelSyncTest(ctx, false, nil, false)
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

	Context("When adding the StorageOS driver registration to a node with existing labels", func() {
		node := SetupNodeLabelSyncTest(ctx, false, mixedLabels, false)
		It("Should sync labels to StorageOS Node", func() {
			By("Expecting StorageOS Node labels to be empty")
			Consistently(func() map[string]string {
				labels, err := api.GetNodeLabels(node.Name)
				if err != nil {
					return nil
				}
				return labels
			}, duration, interval).Should(BeEmpty())

			By("By adding the StorageOS annotation")
			k, v := getCSIAnnotation()
			node.Annotations = map[string]string{
				k: v,
			}
			Expect(k8sClient.Update(ctx, node, &client.UpdateOptions{})).Should(Succeed())

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

	Context("When starting after a k8s Node with labels has been created", func() {
		node := SetupNodeLabelSyncTest(ctx, true, mixedLabels, true)
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
