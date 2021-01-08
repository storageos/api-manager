// Package annotation contains helpers for working with Kubernetes Annotations.
package annotation

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_IncludesStorageOSDriver(t *testing.T) {
	tests := []struct {
		name    string
		obj     client.Object
		want    bool
		wantErr bool
	}{
		{
			name: "no annotations",
			obj:  &corev1.Node{},
			want: false,
		},
		{
			name: "no csi annotations",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
		{
			name: "no storageos csi annotation",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":               "bar",
						DriverAnnotationKey: "{\"csi.xyz.com\":\"f4bfe4d3-fed0-47f0-bc89-e983670f25a9\"}",
					},
				},
			},
			want: false,
		},
		{
			name: "storageos csi annotation",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":               "bar",
						DriverAnnotationKey: "{\"csi.storageos.com\":\"f4bfe4d3-fed0-47f0-bc89-e983670f25a9\"}",
					},
				},
			},
			want: true,
		},
		{
			name: "badly formatted csi annotation",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo":               "bar",
						DriverAnnotationKey: "{\"csi.storageos.com\":}",
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "not a node",
			obj:  &corev1.PersistentVolume{},
			want: false,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := StorageOSCSIDriver(tt.obj)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("StorageOSCSIDriver() error = %v, wantErr %t", gotErr, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("StorageOSCSIDriver() = %v, want %v", got, tt.want)
			}
		})
	}
}
