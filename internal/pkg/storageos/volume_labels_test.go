package storageos_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/storageos/api-manager/internal/pkg/storageos"
	"github.com/storageos/api-manager/internal/pkg/storageos/mocks"
	api "github.com/storageos/go-api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestClient_EnsureVolumeLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		prepare func(key client.ObjectKey, m *mocks.MockControlPlane)
		wantErr bool
	}{
		{
			name: "add unrestricted label",
			labels: map[string]string{
				"foo": "bar",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{
						"foo": "bar",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove unrestricted label",
			labels: map[string]string{},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						"foo": "bar",
					},
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
		},
		{
			name: "add replicas label",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "2",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}
				replicasData := api.SetReplicasRequest{
					Replicas: 2,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "2",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().SetReplicas(gomock.Any(), nsId, volId, replicasData, nil).Return(api.AcceptedMessage{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
		},
		{
			name: "change existing replicas label",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "3",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "2",
					},
				}
				replicasData := api.SetReplicasRequest{
					Replicas: 3,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "3",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().SetReplicas(gomock.Any(), nsId, volId, replicasData, nil).Return(api.AcceptedMessage{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove existing replicas label",
			labels: map[string]string{},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "2",
					},
				}
				replicasData := api.SetReplicasRequest{
					Replicas: 0,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "0",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().SetReplicas(gomock.Any(), nsId, volId, replicasData, nil).Return(api.AcceptedMessage{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
		},
		{
			name: "add replicas label with non integer value",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "not-an-integer",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
			},
			wantErr: true,
		},
		{
			name: "add mixed labels",
			labels: map[string]string{
				"foo":                           "bar",
				"boo":                           "baz",
				storageos.ReservedLabelReplicas: "2",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}
				replicasData := api.SetReplicasRequest{
					Replicas: 2,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "2",
					},
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{
						"foo":                           "bar",
						"boo":                           "baz",
						storageos.ReservedLabelReplicas: "2",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().SetReplicas(gomock.Any(), nsId, volId, replicasData, nil).Return(api.AcceptedMessage{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
		},
		{
			name: "add bad reserved label with unreserved labels",
			labels: map[string]string{
				"foo":                              "bar",
				"boo":                              "baz",
				storageos.ReservedLabelComputeOnly: "2", // compute-only not allowed on volumes
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{
						"foo": "bar",
						"boo": "baz",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
			wantErr: true,
		},
		{
			name: "add computeonly label",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
			},
			wantErr: true,
		},
		{
			name: "add nocache label",
			labels: map[string]string{
				storageos.ReservedLabelNoCache: "true",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
			},
			wantErr: true,
		},
		{
			name: "add nocompress label",
			labels: map[string]string{
				storageos.ReservedLabelNoCompress: "true",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(2)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockCP := mocks.NewMockControlPlane(mockCtrl)

			c := storageos.NewTestAPIClient(mockCP)

			pvcName := "testpvc"
			pvcNamespace := "testns"

			key := client.ObjectKey{Name: pvcName, Namespace: pvcNamespace}
			if tt.prepare != nil {
				tt.prepare(key, mockCP)
			}

			if err := c.EnsureVolumeLabels(context.Background(), key, tt.labels); (err != nil) != tt.wantErr {
				t.Errorf("Client.EnsureVolumeLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_EnsureUnreservedVolumeLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		prepare func(key client.ObjectKey, m *mocks.MockControlPlane)
		wantErr bool
	}{
		{
			name: "add unrestricted label",
			labels: map[string]string{
				"foo": "bar",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{
						"foo": "bar",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
		},
		{
			name: "change unrestricted label",
			labels: map[string]string{
				"foo": "baz",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						"foo": "bar",
					},
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{
						"foo": "baz",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove unrestricted label",
			labels: map[string]string{},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						"foo": "bar",
					},
				}
				updateData := api.UpdateVolumeData{
					Labels: map[string]string{},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
				m.EXPECT().UpdateVolume(gomock.Any(), nsId, volId, updateData, nil).Return(api.Volume{}, nil, nil).Times(1)
			},
		},
		// Restricted label changes are handled by other Ensure functions.  Just
		// check no updates are made and no errors when changes are passed.
		{
			name: "add restricted label - nil existing labels",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "1",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
			},
		},
		{
			name: "add restricted label - empty existing labels",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "1",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels:      map[string]string{},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
			},
		},
		{
			name: "change restricted label",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "2",
			},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "1",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove unrestricted label",
			labels: map[string]string{},
			prepare: func(key client.ObjectKey, m *mocks.MockControlPlane) {
				nsId := uuid.New().String()
				volId := uuid.New().String()
				ns := api.Namespace{
					Id:   nsId,
					Name: key.Namespace,
				}
				vol := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelReplicas: "1",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(1)
			},
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockCP := mocks.NewMockControlPlane(mockCtrl)

			c := storageos.NewTestAPIClient(mockCP)

			pvcName := "testpvc"
			pvcNamespace := "testns"

			key := client.ObjectKey{Name: pvcName, Namespace: pvcNamespace}
			if tt.prepare != nil {
				tt.prepare(key, mockCP)
			}
			if err := c.EnsureUnreservedVolumeLabels(context.Background(), key, tt.labels); (err != nil) != tt.wantErr {
				t.Errorf("Client.EnsureUnreservedVolumeLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
