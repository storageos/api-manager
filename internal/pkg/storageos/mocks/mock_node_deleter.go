// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/storageos/api-manager/internal/pkg/storageos (interfaces: NodeDeleter)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	storageos "github.com/storageos/api-manager/internal/pkg/storageos"
	types "k8s.io/apimachinery/pkg/types"
)

// MockNodeDeleter is a mock of NodeDeleter interface.
type MockNodeDeleter struct {
	ctrl     *gomock.Controller
	recorder *MockNodeDeleterMockRecorder
}

// MockNodeDeleterMockRecorder is the mock recorder for MockNodeDeleter.
type MockNodeDeleterMockRecorder struct {
	mock *MockNodeDeleter
}

// NewMockNodeDeleter creates a new mock instance.
func NewMockNodeDeleter(ctrl *gomock.Controller) *MockNodeDeleter {
	mock := &MockNodeDeleter{ctrl: ctrl}
	mock.recorder = &MockNodeDeleterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNodeDeleter) EXPECT() *MockNodeDeleterMockRecorder {
	return m.recorder
}

// DeleteNode mocks base method.
func (m *MockNodeDeleter) DeleteNode(arg0 context.Context, arg1 types.NamespacedName) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNode", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNode indicates an expected call of DeleteNode.
func (mr *MockNodeDeleterMockRecorder) DeleteNode(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNode", reflect.TypeOf((*MockNodeDeleter)(nil).DeleteNode), arg0, arg1)
}

// ListNodes mocks base method.
func (m *MockNodeDeleter) ListNodes(arg0 context.Context) ([]storageos.Object, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListNodes", arg0)
	ret0, _ := ret[0].([]storageos.Object)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListNodes indicates an expected call of ListNodes.
func (mr *MockNodeDeleterMockRecorder) ListNodes(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListNodes", reflect.TypeOf((*MockNodeDeleter)(nil).ListNodes), arg0)
}
