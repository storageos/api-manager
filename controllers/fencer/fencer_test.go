package fencer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	storageosv1 "github.com/storageos/api-manager/api/v1"
)

func TestControllerRequireAction(t *testing.T) {
	log := zap.New(zap.UseDevMode(true), zap.StacktraceLevel(zapcore.PanicLevel)).V(5)

	// Create a scheme for the test controller.
	scheme := runtime.NewScheme()
	require.Nil(t, clientgoscheme.AddToScheme(scheme))
	require.Nil(t, storageosv1.AddToScheme(scheme))

	testNodeName := "foo-node"

	tests := []struct {
		name           string
		stosNodeName   string
		stosNodeHealth storageosv1.NodeHealth
		k8sNodes       []string
		wantResult     bool
	}{
		{
			name:           "stos node healthy",
			stosNodeName:   testNodeName,
			stosNodeHealth: storageosv1.NodeHealthOnline,
			k8sNodes:       []string{testNodeName, "bar-node"},
			wantResult:     false,
		},
		{
			name:           "stos node offline",
			stosNodeName:   testNodeName,
			stosNodeHealth: storageosv1.NodeHealthOffline,
			k8sNodes:       []string{testNodeName, "bar-node"},
			wantResult:     true,
		},
		{
			name:           "stos node unknown",
			stosNodeName:   testNodeName,
			stosNodeHealth: storageosv1.NodeHealthUnknown,
			k8sNodes:       []string{testNodeName, "bar-node"},
			wantResult:     false,
		},
		{
			name:           "no k8s node for stos node",
			stosNodeName:   testNodeName,
			stosNodeHealth: storageosv1.NodeHealthUnknown,
			k8sNodes:       []string{"rand-node", "bar-node"},
			wantResult:     false,
		},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			// Create storageos node and k8s nodes.
			stosNode := &storageosv1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.stosNodeName,
				},
				Status: storageosv1.NodeStatus{
					Health: tt.stosNodeHealth,
				},
			}

			k8sNodes := []client.Object{}
			for _, knode := range tt.k8sNodes {
				k8sNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: knode,
					},
				}
				k8sNodes = append(k8sNodes, k8sNode)
			}

			// Set up a new controller with a fake k8s client.
			cli := fake.NewClientBuilder().WithObjects(k8sNodes...).Build()
			c, err := NewController(cli, nil, scheme, nil, log)
			require.Nil(t, err)

			r, rErr := c.RequireAction(context.TODO(), stosNode)
			require.Nil(t, rErr)

			require.Equal(t, tt.wantResult, r)
		})
	}
}
