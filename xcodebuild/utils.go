package xcodebuild

import (
	"fmt"
	"path/filepath"
	"strconv"

	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
// while booting. So far it seems that a simple retry solves these issues.
const (
	// This boot timeout can happen when running Unit Tests with Xcode Command Line `xcodebuild`.
	timeOutMessageIPhoneSimulator = "iPhoneSimulator: Timed out waiting"
	// This boot timeout can happen when running Xcode (7+) UI tests with Xcode Command Line `xcodebuild`.
	timeOutMessageUITest                     = "Terminating app due to uncaught exception '_XCTestCaseInterruptionException'"
	earlyUnexpectedExit                      = "Early unexpected exit, operation never finished bootstrapping - no restart will be attempted"
	failureAttemptingToLaunch                = "Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl:"
	failedToBackgroundTestRunner             = `Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner.`
	appStateIsStillNotRunning                = `App state is still not running active, state = XCApplicationStateNotRunning`
	appAccessibilityIsNotLoaded              = `UI Testing Failure - App accessibility isn't loaded`
	testRunnerFailedToInitializeForUITesting = `Test runner failed to initialize for UI testing`
	timedOutRegisteringForTestingEvent       = `Timed out registering for testing event accessibility notifications`
	testRunnerNeverBeganExecuting            = `Test runner never began executing tests after launching.`
	failedToOpenTestRunner                   = `Error Domain=FBSOpenApplicationServiceErrorDomain Code=1 "The request to open.*NSLocalizedFailureReason=The request was denied by service delegate \(SBMainWorkspace\)\.`
)

var testRunnerErrorPatterns = []string{
	timeOutMessageIPhoneSimulator,
	timeOutMessageUITest,
	earlyUnexpectedExit,
	failureAttemptingToLaunch,
	failedToBackgroundTestRunner,
	appStateIsStillNotRunning,
	appAccessibilityIsNotLoaded,
	testRunnerFailedToInitializeForUITesting,
	timedOutRegisteringForTestingEvent,
	testRunnerNeverBeganExecuting,
	failedToOpenTestRunner,
}

// TestParams ...
type TestParams struct {
	ProjectPath                    string
	Scheme                         string
	Destination                    string
	TestPlan                       string
	TestOutputDir                  string
	TestRepetitionMode             string
	MaximumTestRepetitions         int
	RelaunchTestsForEachRepetition bool
	XCConfigContent                string
	PerformCleanAction             bool
	RetryTestsOnFailure            bool
	AdditionalOptions              []string
}

func (b *xcodebuild) createXcodebuildTestArgs(params TestParams) ([]string, error) {
	var xcodebuildArgs []string

	fileExtension := filepath.Ext(params.ProjectPath)
	if fileExtension == ".xcodeproj" {
		xcodebuildArgs = append(xcodebuildArgs, "-project", params.ProjectPath)
	} else if fileExtension == ".xcworkspace" {
		xcodebuildArgs = append(xcodebuildArgs, "-workspace", params.ProjectPath)
	}
	xcodebuildArgs = append(xcodebuildArgs, "-scheme", params.Scheme)

	if params.PerformCleanAction {
		xcodebuildArgs = append(xcodebuildArgs, "clean")
	}

	xcodebuildArgs = append(xcodebuildArgs, "test", "-destination", params.Destination)
	if params.TestPlan != "" {
		xcodebuildArgs = append(xcodebuildArgs, "-testPlan", params.TestPlan)
	}
	xcodebuildArgs = append(xcodebuildArgs, "-resultBundlePath", params.TestOutputDir)

	switch params.TestRepetitionMode {
	case TestRepetitionUntilFailure:
		xcodebuildArgs = append(xcodebuildArgs, "-run-tests-until-failure")
	case TestRepetitionRetryOnFailure:
		xcodebuildArgs = append(xcodebuildArgs, "-retry-tests-on-failure")
	}

	if params.TestRepetitionMode != TestRepetitionNone {
		xcodebuildArgs = append(xcodebuildArgs, "-test-iterations", strconv.Itoa(params.MaximumTestRepetitions))
	}

	if params.RelaunchTestsForEachRepetition {
		xcodebuildArgs = append(xcodebuildArgs, "-test-repetition-relaunch-enabled", "YES")
	}

	if params.XCConfigContent != "" {
		xcconfigPath, err := b.xcconfigWriter.Write(params.XCConfigContent)
		if err != nil {
			return nil, err
		}
		xcodebuildArgs = append(xcodebuildArgs, "-xcconfig", xcconfigPath)
	}

	xcodebuildArgs = append(xcodebuildArgs, params.AdditionalOptions...)

	return xcodebuildArgs, nil
}

func (b *xcodebuild) runTest(params TestRunParams) (string, int, error) {
	xcodebuildArgs, err := b.createXcodebuildTestArgs(params.TestParams)
	if err != nil {
		return "", 1, err
	}

	b.logger.Donef("Running the tests...")

	// When the project path input is set to an SPM Package.swift file, we need to execute the xcodebuild command
	// within the working directory of the project. This is optional for regular workspaces and projects,
	// because we use the `-project` flag to point to the .xcproj/xcworkspace, but we do it for consistency.
	workDir := filepath.Dir(params.TestParams.ProjectPath)
	output, testErr := b.xcodeCommandRunner.Run(workDir, xcodebuildArgs, params.LogFormatterOptions)

	if output.ExitCode != 0 {
		fmt.Println("Exit code: ", output.ExitCode)
	}

	if testErr != nil {
		return b.handleTestRunError(params, testRunResult{xcodebuildLog: string(output.RawOut), exitCode: output.ExitCode, err: testErr})
	}

	return string(output.RawOut), output.ExitCode, nil
}

type testRunResult struct {
	xcodebuildLog string
	exitCode      int
	err           error
}

func (b *xcodebuild) cleanOutputDirAndRerunTest(params TestRunParams) (string, int, error) {
	// Clean output directory, otherwise after retry test run, xcodebuild fails with `error: Existing file at -resultBundlePath "..."`
	if err := b.fileManager.RemoveAll(params.TestParams.TestOutputDir); err != nil {
		return "", 1, fmt.Errorf("failed to clean test output directory: %s: %w", params.TestParams.TestOutputDir, err)
	}
	return b.runTest(params)
}

func (b *xcodebuild) handleTestRunError(prevRunParams TestRunParams, prevRunResult testRunResult) (string, int, error) {
	if prevRunParams.RetryOnSwiftPackageResolutionError && prevRunParams.SwiftPackagesPath != "" && isStringFoundInOutput(cache.SwiftPackagesStateInvalid, prevRunResult.xcodebuildLog) {
		b.logger.Warnf("xcode-test", "swift-packages-cache-invalid", nil, "swift packages cache is in an invalid state")
		if err := b.fileManager.RemoveAll(prevRunParams.SwiftPackagesPath); err != nil {
			b.logger.Errorf("failed to remove Swift package caches: %s", err)
			return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
		}

		prevRunParams.RetryOnSwiftPackageResolutionError = false
		return b.cleanOutputDirAndRerunTest(prevRunParams)
	}

	for _, errorPattern := range testRunnerErrorPatterns {
		if isStringFoundInOutput(errorPattern, prevRunResult.xcodebuildLog) {
			b.logger.Warnf("Automatic retry reason found in log: %s", errorPattern)
			if prevRunParams.RetryOnTestRunnerError {
				b.logger.Printf("Automatic retry is enabled - retrying...")

				prevRunParams.TestParams.RetryTestsOnFailure = false
				prevRunParams.RetryOnTestRunnerError = false
				return b.cleanOutputDirAndRerunTest(prevRunParams)
			}

			b.logger.Errorf("Automatic retry is disabled, no more retry, stopping the test!")
			return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
		}
	}

	if prevRunParams.TestParams.RetryTestsOnFailure {
		b.logger.Warnf("Test run failed")
		b.logger.Printf("'Should retry tests on failure?' (should_retry_test_on_fail) is enabled - retrying...")

		prevRunParams.TestParams.RetryTestsOnFailure = false
		prevRunParams.RetryOnTestRunnerError = false
		return b.cleanOutputDirAndRerunTest(prevRunParams)
	}

	return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
}
