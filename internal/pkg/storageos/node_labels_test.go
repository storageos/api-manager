package storageos_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/storageos/api-manager/internal/pkg/storageos"
	"github.com/storageos/api-manager/internal/pkg/storageos/mocks"
	"github.com/storageos/go-api/v2"
)

func TestClient_EnsureNodeLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		prepare func(name string, m *mocks.MockControlPlane)
		wantErr bool
	}{
		{
			name: "add unrestricted label",
			labels: map[string]string{
				"foo": "bar",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{
						"foo": "bar",
					},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().UpdateNode(gomock.Any(), id, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove unrestricted label",
			labels: map[string]string{},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
					Labels: map[string]string{
						"foo": "bar",
					},
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().UpdateNode(gomock.Any(), id, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name: "add computeonly label",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}
				computeonlyData := api.SetComputeOnlyNodeData{
					ComputeOnly: true,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().SetComputeOnly(gomock.Any(), id, computeonlyData, &api.SetComputeOnlyOpts{}).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name: "disable existing computeonly label",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "false",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
					Labels: map[string]string{
						storageos.ReservedLabelComputeOnly: "true",
					},
				}
				computeonlyData := api.SetComputeOnlyNodeData{
					ComputeOnly: false,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().SetComputeOnly(gomock.Any(), id, computeonlyData, &api.SetComputeOnlyOpts{}).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove existing computeonly label",
			labels: map[string]string{},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
					Labels: map[string]string{
						storageos.ReservedLabelComputeOnly: "true",
					},
				}
				computeonlyData := api.SetComputeOnlyNodeData{
					ComputeOnly: false,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().SetComputeOnly(gomock.Any(), id, computeonlyData, &api.SetComputeOnlyOpts{}).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name: "add computeonly label with non boolean value",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "not-a-boolean",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
			},
			wantErr: true,
		},
		{
			name: "add mixed labels",
			labels: map[string]string{
				"foo":                              "bar",
				"boo":                              "baz",
				storageos.ReservedLabelComputeOnly: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}
				computeonlyData := api.SetComputeOnlyNodeData{
					ComputeOnly: true,
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{
						"foo": "bar",
						"boo": "baz",
					},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().SetComputeOnly(gomock.Any(), id, computeonlyData, &api.SetComputeOnlyOpts{}).Return(api.Node{}, nil, nil).Times(1)
				m.EXPECT().UpdateNode(gomock.Any(), id, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name: "add bad reserved label with unreserved labels",
			labels: map[string]string{
				"foo": "bar",
				"boo": "baz",
				storageos.ReservedLabelPrefix + "badlabelname": "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{
						"foo": "bar",
						"boo": "baz",
					},
				}

				// No SetComputeOnly, but unreserved labels should still update.
				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
				m.EXPECT().UpdateNode(gomock.Any(), id, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
			wantErr: true,
		},
		{
			name: "add replicas label",
			labels: map[string]string{
				storageos.ReservedLabelReplicas: "2",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
			},
			wantErr: true,
		},
		{
			name: "add nocache label",
			labels: map[string]string{
				storageos.ReservedLabelNoCache: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
			},
			wantErr: true,
		},
		{
			name: "add nocompress label",
			labels: map[string]string{
				storageos.ReservedLabelNoCompress: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				id := uuid.New().String()
				node := api.Node{
					Id:   id,
					Name: name,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(2)
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

			key := client.ObjectKey{Name: "nodeA"}
			if tt.prepare != nil {
				tt.prepare(key.Name, mockCP)
			}

			if err := c.EnsureNodeLabels(context.Background(), key, tt.labels); (err != nil) != tt.wantErr {
				t.Errorf("Client.EnsureNodeLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_EnsureUnreservedNodeLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  map[string]string
		prepare func(name string, m *mocks.MockControlPlane)
	}{
		{
			name: "add unrestricted label",
			labels: map[string]string{
				"foo": "bar",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:   nodeId,
					Name: name,
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{
						"foo": "bar",
					},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
				m.EXPECT().UpdateNode(gomock.Any(), nodeId, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name: "change unrestricted label",
			labels: map[string]string{
				"foo": "baz",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:   nodeId,
					Name: name,
					Labels: map[string]string{
						"foo": "bar",
					},
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{
						"foo": "baz",
					},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
				m.EXPECT().UpdateNode(gomock.Any(), nodeId, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove unrestricted label",
			labels: map[string]string{},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:   nodeId,
					Name: name,
					Labels: map[string]string{
						"foo": "bar",
					},
				}
				updateData := api.UpdateNodeData{
					Labels: map[string]string{},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
				m.EXPECT().UpdateNode(gomock.Any(), nodeId, updateData).Return(api.Node{}, nil, nil).Times(1)
			},
		},
		// Restricted label changes are handled by other Ensure functions.  Just
		// check no updates are made and no errors when changes are passed.
		{
			name: "add restricted label - nil existing labels",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:   nodeId,
					Name: name,
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
			},
		},
		{
			name: "add restricted label - empty existing labels",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:     nodeId,
					Name:   name,
					Labels: map[string]string{},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
			},
		},
		{
			name: "change restricted label",
			labels: map[string]string{
				storageos.ReservedLabelComputeOnly: "true",
			},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:   nodeId,
					Name: name,
					Labels: map[string]string{
						storageos.ReservedLabelComputeOnly: "false",
					},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
			},
		},
		{
			name:   "remove unrestricted label",
			labels: map[string]string{},
			prepare: func(name string, m *mocks.MockControlPlane) {
				nodeId := uuid.New().String()
				node := api.Node{
					Id:   nodeId,
					Name: name,
					Labels: map[string]string{
						storageos.ReservedLabelComputeOnly: "true",
					},
				}

				m.EXPECT().ListNodes(gomock.Any()).Return([]api.Node{node}, nil, nil).Times(1)
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

			nodeName := "testnode"

			key := client.ObjectKey{Name: nodeName}
			if tt.prepare != nil {
				tt.prepare(key.Name, mockCP)
			}
			if err := c.EnsureUnreservedNodeLabels(context.Background(), key, tt.labels); err != nil {
				t.Errorf("Client.EnsureUnreservedNodeLabels() error = %v, wantErr %v", err, false)
			}
		})
	}
}
