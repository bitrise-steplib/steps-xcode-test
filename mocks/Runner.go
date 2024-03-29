// Code generated by mockery v2.30.16. DO NOT EDIT.

package mocks

import (
	version "github.com/hashicorp/go-version"
	mock "github.com/stretchr/testify/mock"

	xcodecommand "github.com/bitrise-io/go-xcode/v2/xcodecommand"
)

// XcodeCommandRunner is an autogenerated mock type for the Runner type
type XcodeCommandRunner struct {
	mock.Mock
}

// CheckInstall provides a mock function with given fields:
func (_m *XcodeCommandRunner) CheckInstall() (*version.Version, error) {
	ret := _m.Called()

	var r0 *version.Version
	var r1 error
	if rf, ok := ret.Get(0).(func() (*version.Version, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *version.Version); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*version.Version)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Run provides a mock function with given fields: workDir, xcodebuildOpts, logFormatterOpts
func (_m *XcodeCommandRunner) Run(workDir string, xcodebuildOpts []string, logFormatterOpts []string) (xcodecommand.Output, error) {
	ret := _m.Called(workDir, xcodebuildOpts, logFormatterOpts)

	var r0 xcodecommand.Output
	var r1 error
	if rf, ok := ret.Get(0).(func(string, []string, []string) (xcodecommand.Output, error)); ok {
		return rf(workDir, xcodebuildOpts, logFormatterOpts)
	}
	if rf, ok := ret.Get(0).(func(string, []string, []string) xcodecommand.Output); ok {
		r0 = rf(workDir, xcodebuildOpts, logFormatterOpts)
	} else {
		r0 = ret.Get(0).(xcodecommand.Output)
	}

	if rf, ok := ret.Get(1).(func(string, []string, []string) error); ok {
		r1 = rf(workDir, xcodebuildOpts, logFormatterOpts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewXcodeCommandRunner creates a new instance of XcodeCommandRunner. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewXcodeCommandRunner(t interface {
	mock.TestingT
	Cleanup(func())
}) *XcodeCommandRunner {
	mock := &XcodeCommandRunner{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
