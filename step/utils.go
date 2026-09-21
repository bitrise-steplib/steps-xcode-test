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
		CollectTestDiagnostics: collectTestDiagnosticsValue(
			exportCondition(input.CollectSimulatorDiagnostics), xcodeMajorVersion, additionalOptions),
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
		CollectTestDiagnostics:         cfg.CollectTestDiagnostics,
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

// minimumCollectTestDiagnosticsXcodeMajor is the first Xcode version that understands
// -collect-test-diagnostics and collects Simulator diagnostics itself after a failing test run.
const minimumCollectTestDiagnosticsXcodeMajor = 26

// shouldCollectSimulatorDiagnostics decides whether the Step runs its own `simctl diagnose` after the tests.
// Since Xcode 26 xcodebuild collects the same diagnostics into the xcresult after a failing run
// (see collectTestDiagnosticsValue), so the Step does not collect on those Xcode versions.
func shouldCollectSimulatorDiagnostics(condition exportCondition, testFailed bool, xcodeMajorVersion int64) bool {
	if xcodeMajorVersion >= minimumCollectTestDiagnosticsXcodeMajor {
		return false
	}

	return condition == always || (condition == onFailure && testFailed)
}

// Since Xcode 26 xcodebuild collects a simulator sysdiagnose of its own after a failing test run,
// by shelling out to `simctl diagnose --timeout=600`. That is a separate mechanism from the
// diagnostics this Step collects during teardown, and it runs even when the user asked for no
// diagnostics at all. Measured on a two-file SPM package with a single failing test, it added ten
// minutes to the run and 16 MB to the result bundle, and then gave up with
//
//	IDETestOperationsObserverDebug: Failure collecting diagnostics from simulator:
//	Timed out after 600.0 seconds while waiting for a response from the invoked process
//
// It commonly presents as tests "hanging" at the end of the run. So honour the input the Step
// already has: if the user does not want simulator diagnostics, do not let xcodebuild collect
// them either.
//
// Returns an empty string when the option must not be passed.
func collectTestDiagnosticsValue(condition exportCondition, xcodeMajorVersion int64, additionalOptions []string) string {
	// earlier Xcode (or unknown version: 0)
	if xcodeMajorVersion < minimumCollectTestDiagnosticsXcodeMajor {
		return ""
	}

	// An explicit -collect-test-diagnostics in xcodebuild_options wins.
	for _, option := range additionalOptions {
		if option == "-collect-test-diagnostics" {
			return ""
		}
	}

	// project_setting: do not override anything, xcodebuild follows the test plan.
	if condition == projectSetting {
		return ""
	}

	if condition == never {
		return "never"
	}
	return "on-failure"
}
