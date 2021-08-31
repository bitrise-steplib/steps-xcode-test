package xcodebuild

import (
	"github.com/bitrise-io/go-xcode/models"
	"github.com/bitrise-io/go-xcode/utility"
)

// Xcodebuild ....
type Xcodebuild interface {
	RunBuild(buildParams Params, outputTool string) (string, int, error)
	RunTest(params TestRunParams) (string, int, error)
	Version() (Version, error)
}

type xcodebuild struct {
}

// New ...
func New() Xcodebuild {
	return &xcodebuild{}
}

// Params ...
type Params struct {
	Action                    string
	ProjectPath               string
	Scheme                    string
	DeviceDestination         string
	CleanBuild                bool
	DisableIndexWhileBuilding bool
}

type Version models.XcodebuildVersionModel

func (b *xcodebuild) Version() (Version, error) {
	version, err := utility.GetXcodeVersion()
	return Version(version), err
}

// RunBuild ...
func (b *xcodebuild) RunBuild(buildParams Params, outputTool string) (string, int, error) {
	return runBuild(buildParams, outputTool)
}

// RunTest ...
func (b *xcodebuild) RunTest(params TestRunParams) (string, int, error) {
	return runTest(params)
}
