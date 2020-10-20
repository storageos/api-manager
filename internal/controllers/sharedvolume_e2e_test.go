package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/storageos/api-manager/internal/pkg/storageos"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SharedVolume Controller", func() {

	const timeout = time.Second * 90
	const interval = time.Second * 1
	const wait = time.Second * 5
	const nfsPort int32 = 2049
	ctx := context.Background()

	BeforeEach(func() {
		api.Reset()
	})

	AfterEach(func() {
	})

	Context("PVC and SharedVolume added", func() {
		It("Should create service and update SharedVolume", func() {
			v := api.RandomVol()
			key := types.NamespacedName{
				Name:      v.ServiceName,
				Namespace: v.Namespace,
			}

			By("By creating a new PVC")
			pvc := &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.PVCName,
					Namespace: v.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadOnlyMany,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())

			By("Creating SharedVolume")
			Expect(api.Set(v)).ShouldNot(BeNil())

			svc := &corev1.Service{}

			By("Expecting service")
			Eventually(func() error {
				return k8sClient.Get(ctx, key, svc)
			}, timeout, interval).Should(Succeed())

			By("Expecting service with volume id label set to volume id")
			vid, ok := svc.Labels[storageos.VolumeIDLabelName]
			Expect(ok).Should(BeTrue())
			Expect(vid).Should(Equal(v.ID))

			By("Expecting service with ClusterIP")
			Expect(svc.Spec.ClusterIP).ShouldNot(BeEmpty())

			By("Expecting service with port 2049")
			Expect(svc.Spec.Ports[0].Port).Should(Equal(nfsPort))

			By("Expecting volume ExternalEndpoint to match ClusterIP:2049")
			Eventually(func() string {
				sv, err := api.Get(v.ID, v.Namespace)
				if err != nil {
					return ""
				}
				return sv.ExternalEndpoint
			}, timeout, interval).Should(Equal(fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, nfsPort)))
		})

		It("Should create endpoints", func() {
			v := api.RandomVol()
			key := types.NamespacedName{
				Name:      v.ServiceName,
				Namespace: v.Namespace,
			}

			By("By creating a new PVC")
			ctx := ctx
			pvc := &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.PVCName,
					Namespace: v.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadOnlyMany,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())

			By("Creating SharedVolume")
			Expect(api.Set(v)).ShouldNot(BeNil())

			endpoints := &corev1.Endpoints{}

			By("Expecting endpoints")
			Eventually(func() error {
				return k8sClient.Get(ctx, key, endpoints)
			}, timeout, interval).Should(Succeed())

			By("Expecting endpoints with volume id label set to volume id")
			vid, ok := endpoints.Labels[storageos.VolumeIDLabelName]
			Expect(ok).Should(BeTrue())
			Expect(vid).Should(Equal(v.ID))

			By("Expecting endpoints with correct backend ip address")
			Expect(endpoints.Subsets[0].Addresses[0].IP).Should(Equal(v.InternalAddress()))

			By("Expecting endpoints with correct backend port")
			Expect(endpoints.Subsets[0].Ports[0].Port).Should(Equal(int32(v.InternalPort())))
		})
	})

	Context("SharedVolume updated", func() {
		It("Should not change the service", func() {
			v := api.RandomVol()
			key := types.NamespacedName{
				Name:      v.ServiceName,
				Namespace: v.Namespace,
			}

			By("By creating a new PVC")
			ctx := ctx
			pvc := &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.PVCName,
					Namespace: v.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadOnlyMany,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())

			By("Creating SharedVolume")
			Expect(api.Set(v)).ShouldNot(BeNil())

			svc := &corev1.Service{}

			By("Expecting service")
			Eventually(func() error {
				return k8sClient.Get(ctx, key, svc)
			}, timeout, interval).Should(Succeed())

			clusterIP := svc.Spec.ClusterIP

			By("Changing SharedVolume InternalEndpoint")
			Expect(api.Set(&storageos.SharedVolume{
				ID:               v.ID,
				ServiceName:      v.ServiceName,
				PVCName:          v.PVCName,
				Namespace:        v.Namespace,
				InternalEndpoint: "5.6.7.8:9999",
			})).ShouldNot(BeNil())

			By("Expecting service frontend not to change")
			Consistently(func() string {
				Expect(k8sClient.Get(ctx, key, svc)).Should(Succeed())
				return svc.Spec.ClusterIP
			}, wait, interval).Should(Equal(clusterIP))
		})
		It("Should update the endpoints", func() {
			v := api.RandomVol()
			key := types.NamespacedName{
				Name:      v.ServiceName,
				Namespace: v.Namespace,
			}

			By("By creating a new PVC")
			ctx := ctx
			pvc := &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.PVCName,
					Namespace: v.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadOnlyMany,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())

			By("Creating SharedVolume")
			Expect(api.Set(v)).ShouldNot(BeNil())

			endpoints := &corev1.Endpoints{}

			By("Expecting endpoints")
			Eventually(func() error {
				return k8sClient.Get(ctx, key, endpoints)
			}, timeout, interval).Should(Succeed())

			By("Changing SharedVolume InternalEndpoint")
			Expect(api.Set(&storageos.SharedVolume{
				ID:               v.ID,
				ServiceName:      v.ServiceName,
				PVCName:          v.PVCName,
				Namespace:        v.Namespace,
				InternalEndpoint: "5.6.7.8:9999",
			})).ShouldNot(BeNil())

			By("Expecting endpoints backend ip address to change")
			Eventually(func() string {
				Expect(k8sClient.Get(ctx, key, endpoints)).Should(Succeed())
				return endpoints.Subsets[0].Addresses[0].IP
			}, timeout, interval).Should(Equal("5.6.7.8"))

			By("Expecting endpoints backend port to change")
			Eventually(func() int32 {
				Expect(k8sClient.Get(ctx, key, endpoints)).Should(Succeed())
				return endpoints.Subsets[0].Ports[0].Port
			}, timeout, interval).Should(Equal(int32(9999)))
		})
	})

	// Skipping Service & Endpoints removal tests - we can't test deletion since
	// we rely on OwnerReferences and built-in garbage collection.  envtest
	// doesn't support this:
	//
	// https://book.kubebuilder.io/reference/envtest.html#testing-considerations:
	// the test control plane will behave differently from “real” clusters, and
	// that might have an impact on how you write tests. One common example is
	// garbage collection; because there are no controllers monitoring built-in
	// resources, objects do not get deleted, even if an OwnerReference is set
	// up.
	//
	// Instead, verify that removal of the SharedVolume doesn't delete the
	// service.  This is important since during failover, the InternalEndpoint
	// will be removed from the Volume object for the length of time taken to
	// re-start the server elsewhere.  If this was to trigger volume deletion,
	// then a new service, with a different ClusterIP would be created, leading
	// to NFS stale filehandle errors on clients.
	Context("SharedVolume removed", func() {
		It("Should not delete the service", func() {
			v := api.RandomVol()
			key := types.NamespacedName{
				Name:      v.ServiceName,
				Namespace: v.Namespace,
			}

			By("By creating a new PVC")
			ctx := ctx
			pvc := &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.PVCName,
					Namespace: v.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadOnlyMany,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())

			By("Creating SharedVolume")
			Expect(api.Set(v)).ShouldNot(BeNil())

			svc := &corev1.Service{}

			By("Expecting service")
			Eventually(func() error {
				return k8sClient.Get(ctx, key, svc)
			}, timeout, interval).Should(Succeed())

			clusterIP := svc.Spec.ClusterIP

			By("Removing SharedVolume InternalEndpoint")
			api.Delete(v.ID, v.Namespace)

			By("Expecting service frontend not to change")
			Consistently(func() string {
				Expect(k8sClient.Get(ctx, key, svc)).Should(Succeed())
				return svc.Spec.ClusterIP
			}, wait, interval).Should(Equal(clusterIP))
		})

		// Endpoints will be deleted when the Service has been deleted.  The
		// service requires a target port, which the endpoint provides.
		It("Should not delete the endpoints", func() {
			v := api.RandomVol()
			key := types.NamespacedName{
				Name:      v.ServiceName,
				Namespace: v.Namespace,
			}

			By("By creating a new PVC")
			ctx := ctx
			pvc := &corev1.PersistentVolumeClaim{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.PVCName,
					Namespace: v.Namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadOnlyMany,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).Should(Succeed())

			By("Creating SharedVolume")
			Expect(api.Set(v)).ShouldNot(BeNil())

			endpoints := &corev1.Endpoints{}

			By("Expecting endpoints")
			Eventually(func() error {
				return k8sClient.Get(ctx, key, endpoints)
			}, timeout, interval).Should(Succeed())

			By("Removing SharedVolume InternalEndpoint")
			api.Delete(v.ID, v.Namespace)

			By("Expecting endpoints not to be deleted")
			Consistently(func() error {
				return k8sClient.Get(ctx, key, endpoints)
			}, wait, interval).Should(Succeed())
		})
	})
})
