package xcodebuild

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/progress"
	cache "github.com/bitrise-io/go-xcode/xcodecache"
	"github.com/kballard/go-shellquote"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.
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

var xcodeCommandEnvs = []string{"NSUnbufferedIO=YES"}

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
	BuildParams                    Params
	TestPlan                       string
	TestOutputDir                  string
	TestRepetitionMode             string
	MaximumTestRepetitions         int
	RelaunchTestsForEachRepetition bool
	XCConfigContent                string
	PerformCleanAction             bool
	RetryTestsOnFailure            bool
	AdditionalOptions              string
}

func (b *xcodebuild) runXcodebuildCmd(args ...string) (string, int, error) {
	var outBuffer bytes.Buffer

	cmd := b.commandFactory.Create("xcodebuild", args, &command.Opts{
		Stdout: &outBuffer,
		Stderr: &outBuffer,
		Env:    xcodeCommandEnvs,
	})

	b.logger.Printf("$ %s", cmd.PrintableCommandArgs())
	b.logger.Println()

	var err error
	var exitCode int
	progress.SimpleProgress(".", time.Minute, func() {
		exitCode, err = cmd.RunAndReturnExitCode()
	})

	return outBuffer.String(), exitCode, err
}

func (b *xcodebuild) runPrettyXcodebuildCmd(useStdOut bool, xcprettyArgs []string, xcodebuildArgs []string) (string, int, error) {
	// build outputs:
	// - write it into a buffer
	// - write it into the pipe, which will be fed into xcpretty
	var buildOutBuffer bytes.Buffer
	pipeReader, pipeWriter := io.Pipe()
	buildOutWriters := []io.Writer{pipeWriter}
	buildOutWriter := CreateBufferedWriter(&buildOutBuffer, buildOutWriters...)
	//
	var prettyOutWriter io.Writer
	if useStdOut {
		prettyOutWriter = os.Stdout
	}

	buildCmd := b.commandFactory.Create("xcodebuild", xcodebuildArgs, &command.Opts{
		Stdout: buildOutWriter,
		Stderr: buildOutWriter,
		Env:    xcodeCommandEnvs,
	})

	prettyCmd := b.commandFactory.Create("xcpretty", xcprettyArgs, &command.Opts{
		Stdin:  pipeReader,
		Stdout: prettyOutWriter,
		Stderr: prettyOutWriter,
	})

	b.logger.Printf("$ set -o pipefail && %s | %v", buildCmd.PrintableCommandArgs(), prettyCmd.PrintableCommandArgs())
	b.logger.Println()

	if err := buildCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}
	if err := prettyCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}

	defer func() {
		if err := pipeWriter.Close(); err != nil {
			b.logger.Warnf("Failed to close xcodebuild-xcpretty pipe, error: %s", err)
		}

		if err := prettyCmd.Wait(); err != nil {
			b.logger.Warnf("xcpretty command failed, error: %s", err)
		}
	}()

	if err := buildCmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return buildOutBuffer.String(), 1, errors.New("failed to cast exit status")
			}
			return buildOutBuffer.String(), waitStatus.ExitStatus(), err
		}
		return buildOutBuffer.String(), 1, err
	}

	return buildOutBuffer.String(), 0, nil
}

func (b *xcodebuild) createXcodebuildTestArgs(params TestParams) ([]string, error) {
	buildParams := params.BuildParams

	xcodebuildArgs := []string{buildParams.Action, buildParams.ProjectPath, "-scheme", buildParams.Scheme}
	if params.PerformCleanAction {
		xcodebuildArgs = append(xcodebuildArgs, "clean")
	}

	// Disable indexing during the build.
	// Indexing is needed for autocomplete, ability to quickly jump to definition, get class and method help by alt clicking.
	// Which are not needed in CI environment.
	if buildParams.DisableIndexWhileBuilding {
		xcodebuildArgs = append(xcodebuildArgs, "COMPILER_INDEX_STORE_ENABLE=NO")
	}

	xcodebuildArgs = append(xcodebuildArgs, "test", "-destination", buildParams.DeviceDestination)
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

	if params.AdditionalOptions != "" {
		options, err := shellquote.Split(params.AdditionalOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to parse additional options (%s), error: %s", params.AdditionalOptions, err)
		}
		xcodebuildArgs = append(xcodebuildArgs, options...)
	}

	return xcodebuildArgs, nil
}

func (b *xcodebuild) createXCPrettyArgs(options string) ([]string, error) {
	var args []string

	if options != "" {
		options, err := shellquote.Split(options)
		if err != nil {
			return nil, fmt.Errorf("failed to parse additional options (%s), error: %s", options, err)
		}
		// get and delete the xcpretty output file, if exists
		xcprettyOutputFilePath := ""
		isNextOptOutputPth := false
		for _, aOpt := range options {
			if isNextOptOutputPth {
				xcprettyOutputFilePath = aOpt
				break
			}
			if aOpt == "--output" {
				isNextOptOutputPth = true
				continue
			}
		}
		if xcprettyOutputFilePath != "" {
			if isExist, err := b.pathChecker.IsPathExists(xcprettyOutputFilePath); err != nil {
				b.logger.Errorf("Failed to check xcpretty output file status (path: %s), error: %s", xcprettyOutputFilePath, err)
			} else if isExist {
				b.logger.Warnf("=> Deleting existing xcpretty output: %s", xcprettyOutputFilePath)
				if err := b.fileRemover.Remove(xcprettyOutputFilePath); err != nil {
					b.logger.Errorf("Failed to delete xcpretty output file (path: %s), error: %s", xcprettyOutputFilePath, err)
				}
			}
		}
		//
		args = append(args, options...)
	}

	return args, nil
}

func (b *xcodebuild) runTest(params TestRunParams) (string, int, error) {
	xcodebuildArgs, err := b.createXcodebuildTestArgs(params.BuildTestParams)
	if err != nil {
		return "", 1, err
	}

	b.logger.Infof("Running the tests...")

	var rawOutput string
	var exit int
	var testErr error
	if params.LogFormatter == XcprettyTool {
		xcprettyArgs, err := b.createXCPrettyArgs(params.XcprettyOptions)
		if err != nil {
			return "", 1, err
		}

		rawOutput, exit, testErr = b.runPrettyXcodebuildCmd(true, xcprettyArgs, xcodebuildArgs)
	} else {
		rawOutput, exit, testErr = b.runXcodebuildCmd(xcodebuildArgs...)
	}

	fmt.Println("exit: ", exit)

	if testErr != nil {
		return b.handleTestRunError(params, testRunResult{xcodebuildLog: rawOutput, exitCode: exit, err: testErr})
	}

	return rawOutput, exit, nil
}

type testRunResult struct {
	xcodebuildLog string
	exitCode      int
	err           error
}

func (b *xcodebuild) cleanOutputDirAndRerunTest(params TestRunParams) (string, int, error) {
	// Clean output directory, otherwise after retry test run, xcodebuild fails with `error: Existing file at -resultBundlePath "..."`
	if err := b.fileRemover.RemoveAll(params.BuildTestParams.TestOutputDir); err != nil {
		return "", 1, fmt.Errorf("failed to clean test output directory: %s, error: %s", params.BuildTestParams.TestOutputDir, err)
	}
	return b.runTest(params)
}

func (b *xcodebuild) handleTestRunError(prevRunParams TestRunParams, prevRunResult testRunResult) (string, int, error) {
	if prevRunParams.RetryOnSwiftPackageResolutionError && prevRunParams.SwiftPackagesPath != "" && isStringFoundInOutput(cache.SwiftPackagesStateInvalid, prevRunResult.xcodebuildLog) {
		log.RWarnf("xcode-test", "swift-packages-cache-invalid", nil, "swift packages cache is in an invalid state")
		if err := b.fileRemover.RemoveAll(prevRunParams.SwiftPackagesPath); err != nil {
			b.logger.Errorf("failed to remove Swift package caches, error: %s", err)
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

				prevRunParams.BuildTestParams.RetryTestsOnFailure = false
				prevRunParams.RetryOnTestRunnerError = false
				return b.cleanOutputDirAndRerunTest(prevRunParams)
			}

			b.logger.Errorf("Automatic retry is disabled, no more retry, stopping the test!")
			return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
		}
	}

	if prevRunParams.BuildTestParams.RetryTestsOnFailure {
		b.logger.Warnf("Test run failed")
		b.logger.Printf("'Should retry tests on failure?' (should_retry_test_on_fail) is enabled - retrying...")

		prevRunParams.BuildTestParams.RetryTestsOnFailure = false
		prevRunParams.RetryOnTestRunnerError = false
		return b.cleanOutputDirAndRerunTest(prevRunParams)
	}

	return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
}

func isStringFoundInOutput(searchStr, outputToSearchIn string) bool {
	r := regexp.MustCompile("(?i)" + searchStr)
	return r.MatchString(outputToSearchIn)
}

// CreateBufferedWriter ...
func CreateBufferedWriter(buff *bytes.Buffer, writers ...io.Writer) io.Writer {
	if len(writers) > 0 {
		allWriters := append([]io.Writer{buff}, writers...)
		return io.MultiWriter(allWriters...)
	}
	return io.Writer(buff)
}
