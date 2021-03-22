package storageos

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	api "github.com/storageos/go-api/v2"

	"github.com/storageos/api-manager/internal/pkg/storageos/mocks"
)

func TestClient_Refresh(t *testing.T) {
	secretPath, err := ioutil.TempDir("", "secrets")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(secretPath)
	if err := ioutil.WriteFile(filepath.Join(secretPath, "username"), []byte("username"), 0644); err != nil {
		t.Fatalf("failed to write username file: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(secretPath, "password"), []byte("password"), 0644); err != nil {
		t.Fatalf("failed to write password file: %v", err)
	}

	token := "faketoken"

	errRefresh := errors.New("refresh error")
	errAuth := errors.New("auth error")

	tests := []struct {
		name       string
		ctx        context.Context
		interval   time.Duration
		timeout    time.Duration
		resets     int
		expect     func(ctx context.Context, m *mocks.MockControlPlane)
		wantErrors map[error]int
	}{
		{
			name:     "standard refresh",
			ctx:      context.WithValue(context.Background(), api.ContextAccessToken, token),
			interval: 100 * time.Millisecond,
			timeout:  1050 * time.Millisecond,
			expect: func(ctx context.Context, m *mocks.MockControlPlane) {
				httpResp := &http.Response{
					Header: http.Header{
						"Authorization": []string{"Bearer " + token},
					},
					Body: ioutil.NopCloser(bytes.NewReader([]byte(""))),
				}
				m.EXPECT().RefreshJwt(gomock.Any()).Return(api.UserSession{}, httpResp, nil).Times(10)
			},
		},
		{
			name:     "recovery from refresh error",
			ctx:      context.WithValue(context.Background(), api.ContextAccessToken, token),
			interval: 100 * time.Millisecond,
			timeout:  1050 * time.Millisecond,
			expect: func(ctx context.Context, m *mocks.MockControlPlane) {
				httpResp := &http.Response{
					Header: http.Header{
						"Authorization": []string{"Bearer newtoken"},
					},
					Body: ioutil.NopCloser(bytes.NewReader([]byte(""))),
				}
				m.EXPECT().RefreshJwt(gomock.Any()).Return(api.UserSession{}, nil, errRefresh).Times(1)
				m.EXPECT().AuthenticateUser(gomock.Any(), api.AuthUserData{Username: "username", Password: "password"}).Return(api.UserSession{}, httpResp, nil).Times(1)
				m.EXPECT().RefreshJwt(gomock.Any()).Return(api.UserSession{}, httpResp, nil).Times(9)
			},
			wantErrors: map[error]int{
				errRefresh: 1,
			},
		},
		{
			name:     "external reset",
			ctx:      context.WithValue(context.Background(), api.ContextAccessToken, token),
			interval: 1 * time.Minute,
			timeout:  time.Second,
			resets:   5,
			expect: func(ctx context.Context, m *mocks.MockControlPlane) {
				httpResp := &http.Response{
					Header: http.Header{
						"Authorization": []string{"Bearer newtoken"},
					},
					Body: ioutil.NopCloser(bytes.NewReader([]byte(""))),
				}
				m.EXPECT().AuthenticateUser(gomock.Any(), api.AuthUserData{Username: "username", Password: "password"}).Return(api.UserSession{}, httpResp, nil).Times(5)
			},
		},
		{
			name:     "auth failures",
			ctx:      context.WithValue(context.Background(), api.ContextAccessToken, token),
			interval: 100 * time.Millisecond,
			timeout:  1050 * time.Millisecond,
			expect: func(ctx context.Context, m *mocks.MockControlPlane) {
				httpResp := &http.Response{
					Body: ioutil.NopCloser(bytes.NewReader([]byte(""))),
				}
				m.EXPECT().RefreshJwt(gomock.Any()).Return(api.UserSession{}, nil, errRefresh).Times(10)
				m.EXPECT().AuthenticateUser(gomock.Any(), api.AuthUserData{Username: "username", Password: "password"}).Return(api.UserSession{}, httpResp, errors.New("auth error")).Times(10)
			},
			wantErrors: map[error]int{
				errRefresh: 10,
			},
		},
		{
			name:     "auth failures with resets",
			ctx:      context.WithValue(context.Background(), api.ContextAccessToken, token),
			interval: 100 * time.Millisecond,
			timeout:  1050 * time.Millisecond,
			resets:   5,
			expect: func(ctx context.Context, m *mocks.MockControlPlane) {
				httpResp := &http.Response{
					Body: ioutil.NopCloser(bytes.NewReader([]byte(""))),
				}
				m.EXPECT().RefreshJwt(gomock.Any()).Return(api.UserSession{}, nil, errRefresh).Times(10)
				m.EXPECT().AuthenticateUser(gomock.Any(), api.AuthUserData{Username: "username", Password: "password"}).Return(api.UserSession{}, httpResp, errAuth).Times(15)
			},
			wantErrors: map[error]int{
				errRefresh: 10,
				errAuth:    15,
			},
		},
	}
	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockCP := mocks.NewMockControlPlane(mockCtrl)

			ctx, cancel := context.WithTimeout(tt.ctx, tt.timeout)
			defer cancel()

			errCounter := &TestCounter{Results: make(map[error]int)}

			c := &Client{
				api:       mockCP,
				transport: http.DefaultTransport,
				ctx:       ctx,
				traced:    true,
			}

			if tt.expect != nil {
				tt.expect(ctx, mockCP)
			}

			resetCh := make(chan struct{})

			log := logr.Discard() // zap.New() for output

			go func() {
				if err := c.Refresh(ctx, secretPath, resetCh, tt.interval, errCounter, log); err != ctx.Err() {
					t.Errorf("unexpected error: %v", err)
				}
			}()

			go func() {
				for i := 0; i < tt.resets; i++ {
					select {
					case resetCh <- struct{}{}:
						log.Info("sent reset")
					case <-ctx.Done():
						t.Error("test timed out before all resets could be sent - deadlock?")
					}
				}
			}()

			<-ctx.Done()

			for k, v := range tt.wantErrors {
				if errCounter.Results[k] != v {
					t.Errorf("unexpected error count for %v, got: %d, wnt: %d", k, errCounter.Results[k], v)
				}
			}
		})
	}
}

type TestCounter struct {
	Results map[error]int
	mu      sync.Mutex
}

func (c *TestCounter) Increment(function string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Results[err]++
}
