package xcodebuild

import (
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/models"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-steplib/steps-xcode-test/xcconfig"
)

// Output tools ...
const (
	XcodebuildTool = "xcodebuild"
	XcprettyTool   = "xcpretty"
)

// Test repetition modes ...
const (
	TestRepetitionNone           = "none"
	TestRepetitionUntilFailure   = "until_failure"
	TestRepetitionRetryOnFailure = "retry_on_failure"
)

// Xcodebuild ....
type Xcodebuild interface {
	RunTest(params TestRunParams) (string, int, error)
	Version() (Version, error)
}

type xcodebuild struct {
	logger         log.Logger
	commandFactory command.Factory
	pathChecker    pathutil.PathChecker
	fileManager    fileutil.FileManager
	xcconfigWriter xcconfig.Writer
}

// NewXcodebuild ...
func NewXcodebuild(logger log.Logger, commandFactory command.Factory, pathChecker pathutil.PathChecker, fileManager fileutil.FileManager, xcconfigWriter xcconfig.Writer) Xcodebuild {
	return &xcodebuild{
		logger:         logger,
		commandFactory: commandFactory,
		pathChecker:    pathChecker,
		fileManager:    fileManager,
		xcconfigWriter: xcconfigWriter,
	}
}

// Version ...
type Version models.XcodebuildVersionModel

func (b *xcodebuild) Version() (Version, error) {
	version, err := utility.GetXcodeVersion(b.commandFactory)
	return Version(version), err
}

// Params ...
type Params struct {
	Action      string
	ProjectPath string
	Scheme      string
	Destination string
}

// TestRunParams ...
type TestRunParams struct {
	BuildTestParams                    TestParams
	LogFormatter                       string
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
