// Code generated by mockery v2.30.16. DO NOT EDIT.

package mocks

import (
	io "io"
	fs "io/fs"

	os "os"

	mock "github.com/stretchr/testify/mock"
)

// FileManager is an autogenerated mock type for the FileManager type
type FileManager struct {
	mock.Mock
}

// Open provides a mock function with given fields: path
func (_m *FileManager) Open(path string) (*os.File, error) {
	ret := _m.Called(path)

	var r0 *os.File
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (*os.File, error)); ok {
		return rf(path)
	}
	if rf, ok := ret.Get(0).(func(string) *os.File); ok {
		r0 = rf(path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*os.File)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// OpenReaderIfExists provides a mock function with given fields: path
func (_m *FileManager) OpenReaderIfExists(path string) (io.Reader, error) {
	ret := _m.Called(path)

	var r0 io.Reader
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (io.Reader, error)); ok {
		return rf(path)
	}
	if rf, ok := ret.Get(0).(func(string) io.Reader); ok {
		r0 = rf(path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.Reader)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReadDirEntryNames provides a mock function with given fields: path
func (_m *FileManager) ReadDirEntryNames(path string) ([]string, error) {
	ret := _m.Called(path)

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]string, error)); ok {
		return rf(path)
	}
	if rf, ok := ret.Get(0).(func(string) []string); ok {
		r0 = rf(path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Remove provides a mock function with given fields: path
func (_m *FileManager) Remove(path string) error {
	ret := _m.Called(path)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(path)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveAll provides a mock function with given fields: path
func (_m *FileManager) RemoveAll(path string) error {
	ret := _m.Called(path)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(path)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Write provides a mock function with given fields: path, value, perm
func (_m *FileManager) Write(path string, value string, perm fs.FileMode) error {
	ret := _m.Called(path, value, perm)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, fs.FileMode) error); ok {
		r0 = rf(path, value, perm)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteBytes provides a mock function with given fields: path, value
func (_m *FileManager) WriteBytes(path string, value []byte) error {
	ret := _m.Called(path, value)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, []byte) error); ok {
		r0 = rf(path, value)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *FileManager) FileSizeInBytes(pth string) (int64, error) {
	ret := _m.Called(pth)

	var r0 int64
	if rf, ok := ret.Get(0).(func(string) int64); ok {
		r0 = rf(pth)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(pth)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewFileManager creates a new instance of FileManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewFileManager(t interface {
	mock.TestingT
	Cleanup(func())
}) *FileManager {
	mock := &FileManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
