package sharedvolume

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestSharedVolumeReconcile(t *testing.T) {
	apiPoll := 100 * time.Millisecond
	k8sPoll := 200 * time.Millisecond
	k8sWait := 1 * time.Second
	timeout := 5 * time.Second

	fooName := "foo"
	fooNamespace := "bar"
	fooUID := types.UID("foo-uid")

	fooPVC := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fooName,
			Namespace: fooNamespace,
			UID:       fooUID,
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

	tests := []struct {
		name         string
		volumes      []*storageos.SharedVolume
		cached       []*storageos.SharedVolume
		pvcs         []client.Object
		wantServices []corev1.Service
		wantVolumes  storageos.SharedVolumeList
		wantCached   []*storageos.SharedVolume
		wantEvents   int
		cacheExpiry  time.Duration
	}{
		{
			name: "New volume",
			volumes: []*storageos.SharedVolume{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			pvcs: []client.Object{fooPVC},
			wantServices: []corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz-service",
						Namespace: "bar",
						Labels: map[string]string{
							"storageos.com/sharedvolume": "1234",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolumeClaim",
								Name:       fooName,
								UID:        fooUID,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeClusterIP,
						ClusterIP: "1.2.3.4",
						Ports: []corev1.ServicePort{
							{
								Name:       "nfs",
								Port:       int32(2049),
								Protocol:   "tcp",
								TargetPort: intstr.FromInt(2049),
							},
						},
					},
				},
			},
			wantCached: []*storageos.SharedVolume{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					ExternalEndpoint: "1.2.3.4:2049",
				},
			},
			wantVolumes: storageos.SharedVolumeList{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					ExternalEndpoint: "1.2.3.4:2049",
				},
			},
			wantEvents: 1,
		},
		{
			name: "New volume - PVC not found",
			volumes: []*storageos.SharedVolume{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			pvcs:         []client.Object{},
			wantServices: []corev1.Service{},
			wantCached:   []*storageos.SharedVolume{},
			wantVolumes: storageos.SharedVolumeList{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			wantEvents: 0,
		},
		{
			name: "Cached volume matches - service won't be created",
			pvcs: []client.Object{fooPVC},
			volumes: []*storageos.SharedVolume{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			cached: []*storageos.SharedVolume{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			wantVolumes: storageos.SharedVolumeList{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
		},
		{
			name:        "Cached volume matches but expired",
			pvcs:        []client.Object{fooPVC},
			cacheExpiry: time.Nanosecond,
			volumes: []*storageos.SharedVolume{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			cached: []*storageos.SharedVolume{
				{
					ID:          "1234",
					ServiceName: "baz-service",
					PVCName:     "foo",
					Namespace:   "bar",
				},
			},
			wantServices: []corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz-service",
						Namespace: "bar",
						Labels: map[string]string{
							"storageos.com/sharedvolume": "1234",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolumeClaim",
								Name:       fooName,
								UID:        fooUID,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeClusterIP,
						ClusterIP: "192.168.1.1",
						Ports: []corev1.ServicePort{
							{
								Name:       "nfs",
								Port:       int32(2049),
								Protocol:   "tcp",
								TargetPort: intstr.FromInt(2049),
							},
						},
					},
				},
			},
			// cache entry will expire immediately, so can't check.
			wantCached: []*storageos.SharedVolume{},
			wantVolumes: storageos.SharedVolumeList{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					ExternalEndpoint: "192.168.1.1:2049",
				},
			},
			wantEvents: 1,
		},
		{
			name: "Cached volume no match",
			volumes: []*storageos.SharedVolume{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					InternalEndpoint: "1.2.3.4:1234",
				},
			},
			pvcs: []client.Object{fooPVC},
			cached: []*storageos.SharedVolume{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					InternalEndpoint: "1.2.3.4:5678",
				},
			},
			wantServices: []corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "baz-service",
						Namespace: "bar",
						Labels: map[string]string{
							"storageos.com/sharedvolume": "1234",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolumeClaim",
								Name:       fooName,
								UID:        fooUID,
							},
						},
					},
					Spec: corev1.ServiceSpec{
						Type:      corev1.ServiceTypeClusterIP,
						ClusterIP: "10.10.10.1",
						Ports: []corev1.ServicePort{
							{
								Name:       "nfs",
								Port:       int32(2049),
								Protocol:   "tcp",
								TargetPort: intstr.FromInt(1234),
							},
						},
					},
				},
			},
			wantCached: []*storageos.SharedVolume{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					InternalEndpoint: "1.2.3.4:1234",
					ExternalEndpoint: "10.10.10.1:2049",
				},
			},
			wantVolumes: storageos.SharedVolumeList{
				{
					ID:               "1234",
					ServiceName:      "baz-service",
					PVCName:          "foo",
					Namespace:        "bar",
					InternalEndpoint: "1.2.3.4:1234",
					ExternalEndpoint: "10.10.10.1:2049",
				},
			},
			wantEvents: 1,
		},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			api := &storageos.MockClient{}
			api.Reset()
			for _, v := range tt.volumes {
				api.Set(v)
			}

			s := runtime.NewScheme()
			require.Nil(t, corev1.AddToScheme(s))
			k8s := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.pvcs...).Build()
			recorder := record.NewFakeRecorder(10)

			r := &Reconciler{
				Client:                k8s,
				log:                   ctrl.Log.WithName("unittest"),
				api:                   api,
				apiReset:              make(chan<- struct{}),
				apiPollInterval:       apiPoll,
				cacheExpiryInterval:   tt.cacheExpiry,
				k8sCreatePollInterval: k8sPoll,
				k8sCreateWaitDuration: k8sWait,
				volumes:               cache.New(defaultCacheExpiryInterval, defaultCacheCleanupInterval),
				recorder:              recorder,
			}
			ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

			if tt.cacheExpiry == 0 {
				tt.cacheExpiry = defaultCacheExpiryInterval
			}
			for _, v := range tt.cached {
				if cacheErr := r.volumes.Add(v.ID, v, tt.cacheExpiry); cacheErr != nil {
					t.Errorf("failed to add volume to cache: %v", cacheErr)
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				if err := r.Start(ctx); err != nil && err != context.DeadlineExceeded {
					t.Errorf("SharedVolume.Reconcile() error = %v", err)
				}
				wg.Done()
			}()

			// Wait for services to be created, and add ClusterIP to each.
			for _, wantSvc := range tt.wantServices {
				nn := types.NamespacedName{
					Name:      wantSvc.Name,
					Namespace: wantSvc.Namespace,
				}
				svc := &corev1.Service{}
				if err := r.waitForAvailable(ctx, nn, svc, k8sPoll, k8sWait); err != nil {
					t.Errorf("SharedVolume.Get(svc) error: %v", err)
				}
				if svc.Spec.ClusterIP == "" {
					svc.Spec.ClusterIP = wantSvc.Spec.ClusterIP
				}
				if err := r.Client.Update(ctx, svc, &client.UpdateOptions{}); err != nil {
					t.Errorf("SharedVolume.Update(svc) error: %v", err)
				}
				if !reflect.DeepEqual(svc.GetOwnerReferences(), wantSvc.GetOwnerReferences()) {
					t.Errorf("SharedVolume.Reconcile() svc owner got:\n%v\n, want:\n%v", svc.GetOwnerReferences(), wantSvc.GetOwnerReferences())
				}
			}

			wg.Wait()

			close(recorder.Events)
			if len(recorder.Events) != tt.wantEvents {
				t.Errorf("SharedVolume.Reconcile() events got:\n%v\n, want:\n%v", len(recorder.Events), tt.wantEvents)
			}

			for _, wnt := range tt.wantCached {
				got, found := r.volumes.Get(wnt.ID)
				if !found {
					t.Errorf("SharedVolume.Reconcile() cache expected: %v", wnt)
				}
				if !reflect.DeepEqual(got, wnt) {
					t.Errorf("SharedVolume.Reconcile() cache got:\n%v\n, want:\n%v", got, wnt)
				}
			}
			got, err := r.api.ListSharedVolumes(ctx)
			if err != nil {
				t.Errorf("SharedVolume.ListSharedVolumes() error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantVolumes) {
				t.Errorf("SharedVolume.Reconcile() volumes got:\n%v\n, want:\n%v", got, tt.wantVolumes)
			}
		})
	}
}
