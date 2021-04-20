package storageos_test

import (
	"context"
	"reflect"
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
			},
			wantErr: true,
		},
		{
			name: "add failure-mode label with intent",
			labels: map[string]string{
				storageos.ReservedLabelFailureMode: "soft",
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
				failureModeData := api.SetFailureModeRequest{
					Mode: api.FAILUREMODEINTENT_SOFT,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelFailureMode: "soft",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().SetFailureMode(gomock.Any(), nsId, volId, failureModeData, nil).Return(api.Volume{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
		},
		{
			name: "add failure-mode label with threshold",
			labels: map[string]string{
				storageos.ReservedLabelFailureMode: "2",
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
				failureModeData := api.SetFailureModeRequest{
					FailureThreshold: 2,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelFailureMode: "2",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().SetFailureMode(gomock.Any(), nsId, volId, failureModeData, nil).Return(api.Volume{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
		},
		{
			name: "change existing failure-mode label",
			labels: map[string]string{
				storageos.ReservedLabelFailureMode: "2",
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
						storageos.ReservedLabelFailureMode: "soft",
					},
				}
				failureModeData := api.SetFailureModeRequest{
					FailureThreshold: 2,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelFailureMode: "2",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().SetFailureMode(gomock.Any(), nsId, volId, failureModeData, nil).Return(api.Volume{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove existing failure-mode label",
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
						storageos.ReservedLabelFailureMode: "soft",
					},
				}
				failureModeData := api.SetFailureModeRequest{
					Mode:             api.FAILUREMODEINTENT_HARD,
					FailureThreshold: 0,
				}
				volAfterReservedUpdate := api.Volume{
					Id:          volId,
					NamespaceID: nsId,
					Name:        key.Name,
					Labels: map[string]string{
						storageos.ReservedLabelFailureMode: "",
					},
				}

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
				m.EXPECT().SetFailureMode(gomock.Any(), nsId, volId, failureModeData, nil).Return(api.Volume{}, nil, nil).Times(1)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{volAfterReservedUpdate}, nil, nil).Times(1)
			},
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(2)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
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

				m.EXPECT().ListNamespaces(gomock.Any()).Return([]api.Namespace{ns}, nil, nil).Times(3)
				m.EXPECT().ListVolumes(gomock.Any(), nsId).Return([]api.Volume{vol}, nil, nil).Times(3)
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

func Test_parseFailureMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		s            string
		wantIntent   api.FailureModeIntent
		wantTheshold uint64
		wantErr      bool
	}{
		{
			name:       "soft",
			s:          "soft",
			wantIntent: api.FAILUREMODEINTENT_SOFT,
		},
		{
			name:       "hard",
			s:          "hard",
			wantIntent: api.FAILUREMODEINTENT_HARD,
		},
		{
			name:       "alwayson",
			s:          "alwayson",
			wantIntent: api.FAILUREMODEINTENT_ALWAYSON,
		},
		{
			name:         "threshold 1",
			s:            "1",
			wantTheshold: 1,
		},
		{
			name:       "empty",
			s:          "",
			wantIntent: api.FAILUREMODEINTENT_HARD,
		},
		{
			name:    "invalid",
			s:       "-99",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			intent, thresh, err := storageos.ParseFailureMode(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFailureMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(intent, tt.wantIntent) {
				t.Errorf("parseFailureMode() intent = %v, want %v", intent, tt.wantIntent)
			}
			if thresh != tt.wantTheshold {
				t.Errorf("parseFailureMode() threshold = %v, want %v", thresh, tt.wantTheshold)
			}
		})
	}
}
