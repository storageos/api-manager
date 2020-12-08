package nodedelete

import (
	"testing"
)

func Test_hasStorageOSDriverAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
		wantErr     bool
	}{
		{
			name:        "no annotations",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "no csi annotations",
			annotations: map[string]string{
				"foo": "bar",
			},
			want: false,
		},
		{
			name: "no storageos csi annotation",
			annotations: map[string]string{
				"foo":               "bar",
				DriverAnnotationKey: "{\"csi.xyz.com\":\"f4bfe4d3-fed0-47f0-bc89-e983670f25a9\"}",
			},
			want: false,
		},
		{
			name: "storageos csi annotation",
			annotations: map[string]string{
				"foo":               "bar",
				DriverAnnotationKey: "{\"csi.storageos.com\":\"f4bfe4d3-fed0-47f0-bc89-e983670f25a9\"}",
			},
			want: true,
		},
		{
			name: "badly formatted csi annotation",
			annotations: map[string]string{
				"foo":               "bar",
				DriverAnnotationKey: "{\"csi.storageos.com\":}",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := hasStorageOSDriverAnnotation(tt.annotations)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("hasStorageOSDriverAnnotation() error = %v, wantErr %t", gotErr, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("hasStorageOSDriverAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}
