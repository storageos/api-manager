// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/storageos/api-manager/internal/pkg/storageos (interfaces: Identifier)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockIdentifier is a mock of Identifier interface.
type MockIdentifier struct {
	ctrl     *gomock.Controller
	recorder *MockIdentifierMockRecorder
}

// MockIdentifierMockRecorder is the mock recorder for MockIdentifier.
type MockIdentifierMockRecorder struct {
	mock *MockIdentifier
}

// NewMockIdentifier creates a new mock instance.
func NewMockIdentifier(ctrl *gomock.Controller) *MockIdentifier {
	mock := &MockIdentifier{ctrl: ctrl}
	mock.recorder = &MockIdentifierMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIdentifier) EXPECT() *MockIdentifierMockRecorder {
	return m.recorder
}

// GetID mocks base method.
func (m *MockIdentifier) GetID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetID")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetID indicates an expected call of GetID.
func (mr *MockIdentifierMockRecorder) GetID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetID", reflect.TypeOf((*MockIdentifier)(nil).GetID))
}

// GetName mocks base method.
func (m *MockIdentifier) GetName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetName")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetName indicates an expected call of GetName.
func (mr *MockIdentifierMockRecorder) GetName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetName", reflect.TypeOf((*MockIdentifier)(nil).GetName))
}

// GetNamespace mocks base method.
func (m *MockIdentifier) GetNamespace() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNamespace")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetNamespace indicates an expected call of GetNamespace.
func (mr *MockIdentifierMockRecorder) GetNamespace() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNamespace", reflect.TypeOf((*MockIdentifier)(nil).GetNamespace))
}
