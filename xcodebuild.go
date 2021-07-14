package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/progress"
	cache "github.com/bitrise-io/go-xcode/xcodecache"
	cmd "github.com/bitrise-steplib/steps-xcode-test/command"
	"github.com/bitrise-steplib/steps-xcode-test/models"
	"github.com/kballard/go-shellquote"
)

const (
	none           = "none"
	untilFailure   = "until_failure"
	retryOnFailure = "retry_on_failure"
)

func runXcodebuildCmd(args ...string) (string, int, error) {
	// command
	buildCmd := cmd.CreateXcodebuildCmd(args...)
	// output buffer
	var outBuffer bytes.Buffer
	// set command streams and env
	buildCmd.Stdin = nil
	buildCmd.Stdout = &outBuffer
	buildCmd.Stderr = &outBuffer
	buildCmd.Env = append(os.Environ(), xcodeCommandEnvs...)

	cmdArgsForPrint := cmd.PrintableCommandArgsWithEnvs(buildCmd.Args, xcodeCommandEnvs)

	log.Printf("$ %s", cmdArgsForPrint)

	var err error
	progress.SimpleProgress(".", time.Minute, func() {
		err = buildCmd.Run()
	})
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return outBuffer.String(), 1, errors.New("failed to cast exit status")
			}
			return outBuffer.String(), waitStatus.ExitStatus(), err
		}
		return outBuffer.String(), 1, err
	}
	return outBuffer.String(), 0, nil
}

func runPrettyXcodebuildCmd(useStdOut bool, xcprettyArgs []string, xcodebuildArgs []string) (string, int, error) {
	//
	buildCmd := cmd.CreateXcodebuildCmd(xcodebuildArgs...)
	prettyCmd := cmd.CreateXcprettyCmd(xcprettyArgs...)
	//
	var buildOutBuffer bytes.Buffer
	//
	pipeReader, pipeWriter := io.Pipe()
	//
	// build outputs:
	// - write it into a buffer
	// - write it into the pipe, which will be fed into xcpretty
	buildOutWriters := []io.Writer{pipeWriter}
	buildOutWriter := cmd.CreateBufferedWriter(&buildOutBuffer, buildOutWriters...)
	//
	var prettyOutWriter io.Writer
	if useStdOut {
		prettyOutWriter = os.Stdout
	}

	// and set the writers
	buildCmd.Stdin = nil
	buildCmd.Stdout = buildOutWriter
	buildCmd.Stderr = buildOutWriter
	//
	prettyCmd.Stdin = pipeReader
	prettyCmd.Stdout = prettyOutWriter
	prettyCmd.Stderr = prettyOutWriter
	//
	buildCmd.Env = append(os.Environ(), xcodeCommandEnvs...)

	log.Printf("$ set -o pipefail && %s | %v",
		cmd.PrintableCommandArgsWithEnvs(buildCmd.Args, xcodeCommandEnvs),
		cmd.PrintableCommandArgs(prettyCmd.Args))

	fmt.Println()

	if err := buildCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}
	if err := prettyCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}

	defer func() {
		if err := pipeWriter.Close(); err != nil {
			log.Warnf("Failed to close xcodebuild-xcpretty pipe, error: %s", err)
		}

		if err := prettyCmd.Wait(); err != nil {
			log.Warnf("xcpretty command failed, error: %s", err)
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

func runBuild(buildParams models.XcodebuildParams, outputTool string) (string, int, error) {
	xcodebuildArgs := []string{buildParams.Action, buildParams.ProjectPath, "-scheme", buildParams.Scheme}
	if buildParams.CleanBuild {
		xcodebuildArgs = append(xcodebuildArgs, "clean")
	}

	// Disable indexing during the build.
	// Indexing is needed for autocomplete, ability to quickly jump to definition, get class and method help by alt clicking.
	// Which are not needed in CI environment.
	if buildParams.DisableIndexWhileBuilding {
		xcodebuildArgs = append(xcodebuildArgs, "COMPILER_INDEX_STORE_ENABLE=NO")
	}
	xcodebuildArgs = append(xcodebuildArgs, "build", "-destination", buildParams.DeviceDestination)

	log.Infof("Building the project...")

	if outputTool == xcprettyTool {
		return runPrettyXcodebuildCmd(false, []string{}, xcodebuildArgs)
	}
	return runXcodebuildCmd(xcodebuildArgs...)
}

type testRunParams struct {
	buildTestParams                    models.XcodebuildTestParams
	outputTool                         string
	xcprettyOptions                    string
	retryOnTestRunnerError             bool
	retryOnSwiftPackageResolutionError bool
	swiftPackagesPath                  string
	xcodeMajorVersion                  int
}

type testRunResult struct {
	xcodebuildLog string
	exitCode      int
	err           error
}

func createXCPrettyArgs(options string) ([]string, error) {
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
			if isExist, err := pathutil.IsPathExists(xcprettyOutputFilePath); err != nil {
				log.Errorf("Failed to check xcpretty output file status (path: %s), error: %s", xcprettyOutputFilePath, err)
			} else if isExist {
				log.Warnf("=> Deleting existing xcpretty output: %s", xcprettyOutputFilePath)
				if err := os.Remove(xcprettyOutputFilePath); err != nil {
					log.Errorf("Failed to delete xcpretty output file (path: %s), error: %s", xcprettyOutputFilePath, err)
				}
			}
		}
		//
		args = append(args, options...)
	}

	return args, nil
}

func createXcodebuildTestArgs(params models.XcodebuildTestParams, xcodeMajorVersion int) ([]string, error) {
	buildParams := params.BuildParams

	xcodebuildArgs := []string{buildParams.Action, buildParams.ProjectPath, "-scheme", buildParams.Scheme}
	if params.CleanBuild {
		xcodebuildArgs = append(xcodebuildArgs, "clean")
	}
	// the 'build' argument is required *before* the 'test' arg, to prevent
	//  the Xcode bug described in the README, which causes:
	// 'iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.'
	//  in case the compilation takes a long time.
	// Related Radar link: https://openradar.appspot.com/22413115
	// Demonstration project: https://github.com/bitrise-io/simulator-launch-timeout-includes-build-time

	// for builds < 120 seconds or fixed Xcode versions, one should
	// have the possibility of opting out, because the explicit build arg
	// leads the project to be compiled twice and increase the duration
	// Related issue link: https://github.com/bitrise-steplib/steps-xcode-test/issues/55
	if params.BuildBeforeTest {
		xcodebuildArgs = append(xcodebuildArgs, "build")
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

	if params.GenerateCodeCoverage {
		xcodebuildArgs = append(xcodebuildArgs, "GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES", "GCC_GENERATE_TEST_COVERAGE_FILES=YES")
	}

	switch params.TestRepetitionMode {
	case untilFailure:
		xcodebuildArgs = append(xcodebuildArgs, "-run-tests-until-failure")
	case retryOnFailure:
		xcodebuildArgs = append(xcodebuildArgs, "-retry-tests-on-failure")
	}

	if params.TestRepetitionMode != none {
		xcodebuildArgs = append(xcodebuildArgs, "-test-iterations", strconv.Itoa(params.MaximumTestRepetitions))
	}

	if params.RelaunchTestsForEachRepetition {
		xcodebuildArgs = append(xcodebuildArgs, "-test-repetition-relaunch-enabled", "YES")
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

func handleTestRunError(prevRunParams testRunParams, prevRunResult testRunResult) (string, int, error) {
	if prevRunParams.retryOnSwiftPackageResolutionError && prevRunParams.swiftPackagesPath != "" && isStringFoundInOutput(cache.SwiftPackagesStateInvalid, prevRunResult.xcodebuildLog) {
		log.RWarnf("xcode-test", "swift-packages-cache-invalid", nil, "swift packages cache is in an invalid state")
		if err := os.RemoveAll(prevRunParams.swiftPackagesPath); err != nil {
			log.Errorf("failed to remove Swift package caches, error: %s", err)
			return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
		}

		prevRunParams.retryOnSwiftPackageResolutionError = false
		return cleanOutputDirAndRerunTest(prevRunParams)
	}

	for _, errorPattern := range testRunnerErrorPatterns {
		if isStringFoundInOutput(errorPattern, prevRunResult.xcodebuildLog) {
			log.Warnf("Automatic retry reason found in log: %s", errorPattern)
			if prevRunParams.retryOnTestRunnerError {
				log.Printf("retryOnTestRunnerError=true - retrying...")

				prevRunParams.buildTestParams.RetryTestsOnFailure = false
				prevRunParams.retryOnTestRunnerError = false
				return cleanOutputDirAndRerunTest(prevRunParams)
			}

			log.Errorf("retryOnTestRunnerError=false, no more retry, stopping the test!")
			return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
		}
	}

	if prevRunParams.xcodeMajorVersion < 13 && prevRunParams.buildTestParams.RetryTestsOnFailure {
		log.Warnf("Test run failed")
		log.Printf("retryTestsOnFailure=true - retrying...")

		prevRunParams.buildTestParams.RetryTestsOnFailure = false
		prevRunParams.retryOnTestRunnerError = false
		return cleanOutputDirAndRerunTest(prevRunParams)
	}

	return prevRunResult.xcodebuildLog, prevRunResult.exitCode, prevRunResult.err
}

func cleanOutputDirAndRerunTest(params testRunParams) (string, int, error) {
	// Clean output directory, otherwise after retry test run, xcodebuild fails with `error: Existing file at -resultBundlePath "..."`
	if err := os.RemoveAll(params.buildTestParams.TestOutputDir); err != nil {
		return "", 1, fmt.Errorf("failed to clean test output directory: %s, error: %s", params.buildTestParams.TestOutputDir, err)
	}
	return runTest(params)
}

func runTest(params testRunParams) (string, int, error) {
	xcodebuildArgs, err := createXcodebuildTestArgs(params.buildTestParams, params.xcodeMajorVersion)
	if err != nil {
		return "", 1, err
	}

	log.Infof("Running the tests...")

	var rawOutput string
	var exit int
	var testErr error
	if params.outputTool == xcprettyTool {
		xcprettyArgs, err := createXCPrettyArgs(params.xcprettyOptions)
		if err != nil {
			return "", 1, err
		}

		rawOutput, exit, testErr = runPrettyXcodebuildCmd(true, xcprettyArgs, xcodebuildArgs)
	} else {
		rawOutput, exit, testErr = runXcodebuildCmd(xcodebuildArgs...)
	}

	fmt.Println("exit: ", exit)

	if testErr != nil {
		return handleTestRunError(params, testRunResult{xcodebuildLog: rawOutput, exitCode: exit, err: testErr})
	}

	return rawOutput, exit, nil
}
