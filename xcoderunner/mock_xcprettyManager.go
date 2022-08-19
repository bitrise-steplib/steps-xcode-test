// Code generated by mockery v2.13.1. DO NOT EDIT.

package xcoderunner

import (
	command "github.com/bitrise-io/go-utils/v2/command"
	mock "github.com/stretchr/testify/mock"

	version "github.com/hashicorp/go-version"
)

// mockXcprettyManager is an autogenerated mock type for the xcprettyManager type
type mockXcprettyManager struct {
	mock.Mock
}

// depVersion provides a mock function with given fields:
func (_m *mockXcprettyManager) depVersion() (*version.Version, error) {
	ret := _m.Called()

	var r0 *version.Version
	if rf, ok := ret.Get(0).(func() *version.Version); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*version.Version)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// installDep provides a mock function with given fields:
func (_m *mockXcprettyManager) installDep() []command.Command {
	ret := _m.Called()

	var r0 []command.Command
	if rf, ok := ret.Get(0).(func() []command.Command); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]command.Command)
		}
	}

	return r0
}

// isDepInstalled provides a mock function with given fields:
func (_m *mockXcprettyManager) isDepInstalled() (bool, error) {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTnewMockXcprettyManager interface {
	mock.TestingT
	Cleanup(func())
}

// newMockXcprettyManager creates a new instance of mockXcprettyManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func newMockXcprettyManager(t mockConstructorTestingTnewMockXcprettyManager) *mockXcprettyManager {
	mock := &mockXcprettyManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
