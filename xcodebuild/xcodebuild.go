package xcodebuild

import (
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/models"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-io/go-xcode/v2/xcconfig"
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
	version, err := utility.GetXcodeVersion()
	return Version(version), err
}

// TestRunParams ...
type TestRunParams struct {
	TestParams                         TestParams
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
