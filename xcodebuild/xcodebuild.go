package xcodebuild

import (
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/xcconfig"
	"github.com/bitrise-io/go-xcode/v2/xcodecommand"
)

const (
	TestRepetitionNone           = "none"
	TestRepetitionUntilFailure   = "until_failure"
	TestRepetitionRetryOnFailure = "retry_on_failure"
)

type Xcodebuild interface {
	RunTest(params TestRunParams) (string, int, error)
	GetXcodeCommadRunner() xcodecommand.Runner
	SetXcodeCommandRunner(runner xcodecommand.Runner)
}

type xcodebuild struct {
	logger             log.Logger
	fileManager        fileutil.FileManager
	xcconfigWriter     xcconfig.Writer
	xcodeCommandRunner xcodecommand.Runner
}

func NewXcodebuild(logger log.Logger, fileManager fileutil.FileManager, xcconfigWriter xcconfig.Writer, xcodeCommandRunner xcodecommand.Runner) Xcodebuild {
	return &xcodebuild{
		logger:             logger,
		fileManager:        fileManager,
		xcconfigWriter:     xcconfigWriter,
		xcodeCommandRunner: xcodeCommandRunner,
	}
}

type TestRunParams struct {
	TestParams                         TestParams
	LogFormatterOptions                []string
	RetryOnTestRunnerError             bool
}

func (b *xcodebuild) RunTest(params TestRunParams) (string, int, error) {
	return b.runTest(params)
}

func (b *xcodebuild) GetXcodeCommadRunner() xcodecommand.Runner {
	return b.xcodeCommandRunner
}

func (b *xcodebuild) SetXcodeCommandRunner(runner xcodecommand.Runner) {
	b.xcodeCommandRunner = runner
}
