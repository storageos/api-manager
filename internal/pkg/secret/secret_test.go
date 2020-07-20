package secret

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		data    string
		want    string
		create  bool
		wantErr bool
	}{
		{
			name:    "found",
			path:    "/etc/storageos/api/username",
			data:    "storageos",
			want:    "storageos",
			create:  true,
			wantErr: false,
		},
		{
			name:    "whitespace stripped",
			path:    "/etc/storageos/api/username",
			data:    "   storageos   ",
			want:    "storageos",
			create:  true,
			wantErr: false,
		},
		{
			name:    "not found",
			path:    "/etc/storageos/api/username",
			create:  false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if tt.create {
				tmpDir, err := ioutil.TempDir("", "*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}
				path = filepath.Join(tmpDir, tt.path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatalf("failed to create path dir: %v", err)
				}
				if err := ioutil.WriteFile(path, []byte(tt.data), 0644); err != nil {
					t.Fatalf("failed to write temp file: %v", err)
				}
				defer os.Remove(path)
			}
			got, err := Read(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Read() = %v, want %v", got, tt.want)
			}
		})
	}
}
