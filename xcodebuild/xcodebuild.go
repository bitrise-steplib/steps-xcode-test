package xcodebuild

import (
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/models"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-steplib/steps-xcode-test/fileremover"
)

// Xcodebuild ....
type Xcodebuild interface {
	RunBuild(buildParams Params, outputTool string) (string, int, error)
	RunTest(params TestRunParams) (string, int, error)
	Version() (Version, error)
}

type xcodebuild struct {
	logger         log.Logger
	commandFactory command.Factory
	pathChecker    pathutil.PathChecker
	fileRemover    fileremover.FileRemover
}

// New ...
func New(logger log.Logger, commandFactory command.Factory, pathChecker pathutil.PathChecker, fileRemover fileremover.FileRemover) Xcodebuild {
	return &xcodebuild{
		logger:         logger,
		commandFactory: commandFactory,
		pathChecker:    pathChecker,
		fileRemover:    fileRemover,
	}
}

// Version ...
type Version models.XcodebuildVersionModel

func (b *xcodebuild) Version() (Version, error) {
	version, err := utility.GetXcodeVersion()
	return Version(version), err
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

// RunBuild ...
func (b *xcodebuild) RunBuild(buildParams Params, outputTool string) (string, int, error) {
	return b.runBuild(buildParams, outputTool)
}

// TestRunParams ...
type TestRunParams struct {
	BuildTestParams                    TestParams
	OutputTool                         string
	XcprettyOptions                    string
	RetryOnTestRunnerError             bool
	RetryOnSwiftPackageResolutionError bool
	SwiftPackagesPath                  string
	XcodeMajorVersion                  int
}

// RunTest ...
func (b *xcodebuild) RunTest(params TestRunParams) (string, int, error) {
	return b.runTest(params)
}
