// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/storageos/api-manager/internal/pkg/storageos (interfaces: NamespaceDeleter)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	types "k8s.io/apimachinery/pkg/types"
	reflect "reflect"
)

// MockNamespaceDeleter is a mock of NamespaceDeleter interface
type MockNamespaceDeleter struct {
	ctrl     *gomock.Controller
	recorder *MockNamespaceDeleterMockRecorder
}

// MockNamespaceDeleterMockRecorder is the mock recorder for MockNamespaceDeleter
type MockNamespaceDeleterMockRecorder struct {
	mock *MockNamespaceDeleter
}

// NewMockNamespaceDeleter creates a new mock instance
func NewMockNamespaceDeleter(ctrl *gomock.Controller) *MockNamespaceDeleter {
	mock := &MockNamespaceDeleter{ctrl: ctrl}
	mock.recorder = &MockNamespaceDeleterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockNamespaceDeleter) EXPECT() *MockNamespaceDeleterMockRecorder {
	return m.recorder
}

// DeleteNamespace mocks base method
func (m *MockNamespaceDeleter) DeleteNamespace(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNamespace", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNamespace indicates an expected call of DeleteNamespace
func (mr *MockNamespaceDeleterMockRecorder) DeleteNamespace(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNamespace", reflect.TypeOf((*MockNamespaceDeleter)(nil).DeleteNamespace), arg0, arg1)
}

// ListNamespaces mocks base method
func (m *MockNamespaceDeleter) ListNamespaces(arg0 context.Context) ([]types.NamespacedName, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListNamespaces", arg0)
	ret0, _ := ret[0].([]types.NamespacedName)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListNamespaces indicates an expected call of ListNamespaces
func (mr *MockNamespaceDeleterMockRecorder) ListNamespaces(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListNamespaces", reflect.TypeOf((*MockNamespaceDeleter)(nil).ListNamespaces), arg0)
}
