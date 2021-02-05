package fencer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pkg/errors"
	"github.com/storageos/api-manager/internal/pkg/storageos"
)

func TestReconciler_pollNodes(t *testing.T) {
	log := zap.New(zap.UseDevMode(true), zap.StacktraceLevel(zapcore.PanicLevel)).V(5)
	tests := []struct {
		name          string
		interval      time.Duration
		timeout       time.Duration
		nodes         []storageos.Object
		wantNodes     int
		wantMinEvents int
		listNodesErr  error
		wantAPIErr    bool
	}{
		{
			name:     "one node, one event per interval",
			interval: 100 * time.Millisecond,
			timeout:  1 * time.Second,
			nodes: []storageos.Object{
				storageos.MockObject{
					Name: "nodeA",
				},
			},
			wantNodes:     1,
			wantMinEvents: 9, // should get 10, be safe.
		},
		{
			name:     "three nodes, 3 events per interval",
			interval: 100 * time.Millisecond,
			timeout:  1 * time.Second,
			nodes: []storageos.Object{
				storageos.MockObject{
					Name: "nodeA",
				},
				storageos.MockObject{
					Name: "nodeB",
				},
				storageos.MockObject{
					Name: "nodeC",
				},
			},
			wantNodes:     3,
			wantMinEvents: 25, // should get 30, be safe.
		},
		{
			name:     "listvolumes error",
			interval: 100 * time.Millisecond,
			timeout:  1 * time.Second,
			nodes: []storageos.Object{
				storageos.MockObject{
					Name: "nodeA",
				},
			},
			wantNodes:     0,
			wantMinEvents: 0,
			listNodesErr:  errors.New("not now"),
			wantAPIErr:    true,
		},
		{
			name:     "timeout before first tick",
			interval: time.Minute,
			timeout:  time.Second,
			nodes: []storageos.Object{
				storageos.MockObject{
					Name: "nodeA",
				},
			},
			wantNodes:     0,
			wantMinEvents: 0,
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			var apiResetCh = make(chan struct{})
			var srcCh = make(chan event.GenericEvent)
			var doneCh = make(chan struct{})

			api := storageos.NewMockClient()
			for _, n := range tt.nodes {
				if err := api.AddNode(n); err != nil {
					t.Fatalf("couldn't add node: %v", err)
				}
			}
			if tt.listNodesErr != nil {
				api.ListNodesErr = tt.listNodesErr
			}

			r := &Reconciler{
				Client:       nil,
				scheme:       nil,
				log:          log,
				api:          api,
				apiReset:     apiResetCh,
				pollInterval: tt.interval,
			}

			go func() {
				r.pollNodes(ctx, srcCh)
				close(doneCh)
			}()

			var events = make(map[string]event.GenericEvent)
			var eventCnt int
			var apiErr bool

		results:
			for {
				select {
				case evt := <-srcCh:
					key := client.ObjectKey{Name: evt.Object.GetName()}
					events[key.String()] = evt
					eventCnt++
				case <-apiResetCh:
					apiErr = true
				case <-doneCh:
					break results
				}
			}

			if !tt.wantAPIErr && apiErr {
				t.Fatal("unexpected api reset")
			}
			if tt.wantAPIErr {
				if !apiErr {
					t.Errorf("expected api reset, got none")
				}
				return
			}

			if len(events) != tt.wantNodes {
				t.Errorf("got %d nodes, expected %d", len(events), tt.wantNodes)
			}

			if eventCnt < tt.wantMinEvents {
				t.Errorf("got %d events, expected minimum %d", eventCnt, tt.wantMinEvents)
			}
		})
	}
}

func TestNewReconciler(t *testing.T) {
	testDuration := 13 * time.Second

	tests := []struct {
		name           string
		pollInterval   time.Duration
		expiryInterval time.Duration
		wantReconciler Reconciler
	}{
		{
			name:           "basic",
			pollInterval:   testDuration,
			expiryInterval: testDuration,
			wantReconciler: Reconciler{
				pollInterval:   testDuration,
				expiryInterval: testDuration,
			},
		},
		{
			name:           "low poll interval",
			pollInterval:   2 * time.Second,
			expiryInterval: testDuration,
			wantReconciler: Reconciler{
				pollInterval:   minPollInterval,
				expiryInterval: testDuration,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := NewReconciler(nil, nil, nil, tt.pollInterval, tt.expiryInterval)

			assert.Equal(t, tt.wantReconciler.pollInterval, got.pollInterval)
			assert.Equal(t, tt.wantReconciler.expiryInterval, got.expiryInterval)
		})
	}
}
