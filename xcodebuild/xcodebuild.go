package xcodebuild

import (
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/xcconfig"
	"github.com/bitrise-steplib/steps-xcode-test/xcodecommand"
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
}

type xcodebuild struct {
	logger             log.Logger
	pathChecker        pathutil.PathChecker
	fileManager        fileutil.FileManager
	xcconfigWriter     xcconfig.Writer
	xcodeCommandRunner xcodecommand.Runner
}

// NewXcodebuild ...
func NewXcodebuild(logger log.Logger, pathChecker pathutil.PathChecker, fileManager fileutil.FileManager, xcconfigWriter xcconfig.Writer, xcodeCommandRunner xcodecommand.Runner) Xcodebuild {
	return &xcodebuild{
		logger:             logger,
		pathChecker:        pathChecker,
		fileManager:        fileManager,
		xcconfigWriter:     xcconfigWriter,
		xcodeCommandRunner: xcodeCommandRunner,
	}
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
