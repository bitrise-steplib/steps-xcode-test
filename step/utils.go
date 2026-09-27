package step

import (
	"fmt"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/log/colorstring"
	"github.com/bitrise-io/go-utils/v2/stringutil"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
)

type Utils interface {
	PrintLastLinesOfXcodebuildTestLog(rawXcodebuildOutput string, isRunSuccess bool)
	CreateConfig(input Input, projectPath string, sim destination.Device, additionalOptions, additionalLogFormatterOptions []string, skipTesting []string, xcodeMajorVersion int64) Config
	CreateTestParams(cfg Config, xcresultPath, swiftPackagesPath string) xcodebuild.TestRunParams
}

type utils struct {
	logger log.Logger
}

func NewUtils(logger log.Logger) Utils {
	return &utils{
		logger: logger,
	}
}

func (u utils) PrintLastLinesOfXcodebuildTestLog(rawXcodebuildOutput string, isRunSuccess bool) {
	const lastLines = "\nLast lines of the build log:"
	if !isRunSuccess {
		u.logger.Errorf(lastLines)
	} else {
		u.logger.Infof(lastLines)
	}

	fmt.Println(stringutil.LastNLines(rawXcodebuildOutput, 20))

	if !isRunSuccess {
		u.logger.Warnf("If you can't find the reason of the error in the log, please check the xcodebuild_test.log.")
	}

	u.logger.Infof(colorstring.Magenta(`
The log file is stored in $BITRISE_DEPLOY_DIR, and its full path
is available in the $BITRISE_XCODEBUILD_TEST_LOG_PATH environment variable.

If you have the Deploy to Bitrise.io step (after this step),
that will attach the file to your build as an artifact!`))

}

func (u utils) CreateConfig(input Input,
	projectPath string,
	sim destination.Device,
	additionalOptions, additionalLogFormatterOptions []string, skipTesting []string,
	xcodeMajorVersion int64) Config {
	return Config{
		ProjectPath: projectPath,
		Scheme:      input.Scheme,
		TestPlan:    input.TestPlan,

		Simulator:         sim,
		IsSimulatorBooted: sim.State != simulatorShutdownState,

		TestRepetitionMode:            input.TestRepetitionMode,
		MaximumTestRepetitions:        input.MaximumTestRepetitions,
		RelaunchTestForEachRepetition: input.RelaunchTestsForEachRepetition,

		XCConfigContent:    input.XCConfigContent,
		PerformCleanAction: input.PerformCleanAction,
		XcodebuildOptions:  additionalOptions,

		LogFormatter:        input.LogFormatter,
		LogFormatterOptions: additionalLogFormatterOptions,

		CacheLevel: input.CacheLevel,

		SkipTesting:                 skipTesting,
		CollectSimulatorDiagnostics: exportCondition(input.CollectSimulatorDiagnostics),
		XcodebuildDiagnosticsOverride: xcodebuildDiagnosticsOverride(
			exportCondition(input.CollectSimulatorDiagnostics), xcodeMajorVersion),
		HeadlessMode:      input.HeadlessMode,
		XcodeMajorVersion: xcodeMajorVersion,

		DeployDir: input.DeployDir,
	}
}

func (u utils) CreateTestParams(cfg Config, xcresultPath, swiftPackagesPath string) xcodebuild.TestRunParams {
	testParams := xcodebuild.TestParams{
		ProjectPath:                    cfg.ProjectPath,
		Scheme:                         cfg.Scheme,
		Destination:                    cfg.Simulator.XcodebuildDestination(),
		TestPlan:                       cfg.TestPlan,
		TestOutputDir:                  xcresultPath,
		TestRepetitionMode:             cfg.TestRepetitionMode,
		MaximumTestRepetitions:         cfg.MaximumTestRepetitions,
		RelaunchTestsForEachRepetition: cfg.RelaunchTestForEachRepetition,
		XCConfigContent:                cfg.XCConfigContent,
		PerformCleanAction:             cfg.PerformCleanAction,
		SkipTesting:                    cfg.SkipTesting,
		XcodebuildDiagnosticsOverride:  cfg.XcodebuildDiagnosticsOverride,
		AdditionalOptions:              cfg.XcodebuildOptions,
	}

	return xcodebuild.TestRunParams{
		TestParams:                         testParams,
		LogFormatterOptions:                cfg.LogFormatterOptions,
		RetryOnTestRunnerError:             true,
		RetryOnSwiftPackageResolutionError: true,
		SwiftPackagesPath:                  swiftPackagesPath,
	}
}

// minimumXcodeMajorWithDiagnosticsOption is the first Xcode version that understands
// -collect-test-diagnostics and collects Simulator diagnostics itself after a failing test run.
const minimumXcodeMajorWithDiagnosticsOption = 26

func shouldStepCollectDiagnostics(condition exportCondition, testFailed bool, xcodeMajorVersion int64) bool {
	if xcodeMajorVersion >= minimumXcodeMajorWithDiagnosticsOption { // xcodebuild collects
		return false
	}

	return condition == always || (condition == onFailure && testFailed)
}

func xcodebuildCollectsDiagnostics(condition exportCondition, testFailed bool, xcodeMajorVersion int64) bool {
	return xcodeMajorVersion >= minimumXcodeMajorWithDiagnosticsOption && condition != never && testFailed
}

// xcodebuildDiagnosticsOverride is the -collect-test-diagnostics value the Step passes. A
// -collect-test-diagnostics in xcodebuild_options replaces it (go-xcode reports the
// override), so the user's option still wins.
func xcodebuildDiagnosticsOverride(condition exportCondition, xcodeMajorVersion int64) string {
	if xcodeMajorVersion < minimumXcodeMajorWithDiagnosticsOption { // no such option yet
		return ""
	}

	if condition == projectSetting { // leave it to the test plan
		return ""
	}

	if condition == never {
		return "never"
	}
	return "on-failure"
}
