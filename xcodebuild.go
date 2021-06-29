package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func runXcodeBuildCmd(args ...string) (string, int, error) {
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

func runPrettyXcodeBuildCmd(useStdOut bool, xcprettyArgs []string, xcodebuildArgs []string) (string, int, error) {
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

func runBuild(buildParams models.XcodeBuildParamsModel, outputTool string) (string, int, error) {
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
		return runPrettyXcodeBuildCmd(false, []string{}, xcodebuildArgs)
	}
	return runXcodeBuildCmd(xcodebuildArgs...)
}

func runTest(buildTestParams models.XcodeBuildTestParamsModel, outputTool, xcprettyOptions string, isAutomaticRetryOnReason, isRetryOnFail bool, swiftPackagesPath string) (string, int, error) {
	handleTestError := func(fullOutputStr string, exitCode int, testError error) (string, int, error) {
		if swiftPackagesPath != "" && isStringFoundInOutput(cache.SwiftPackagesStateInvalid, fullOutputStr) {
			log.RWarnf("xcode-test", "swift-packages-cache-invalid", nil, "swift packages cache is in an invalid state")
			if err := os.RemoveAll(swiftPackagesPath); err != nil {
				log.Errorf("failed to remove Swift package caches, error: %s", err)
				return fullOutputStr, exitCode, testError
			}
		}

		//
		// Automatic retry
		for _, retryReasonPattern := range automaticRetryReasonPatterns {
			if isStringFoundInOutput(retryReasonPattern, fullOutputStr) {
				log.Warnf("Automatic retry reason found in log: %s", retryReasonPattern)
				if isAutomaticRetryOnReason {
					log.Printf("isAutomaticRetryOnReason=true - retrying...")
					return runTest(buildTestParams, outputTool, xcprettyOptions, false, false, swiftPackagesPath)
				}
				log.Errorf("isAutomaticRetryOnReason=false, no more retry, stopping the test!")
				return fullOutputStr, exitCode, testError
			}
		}

		//
		// Retry on fail
		if isRetryOnFail {
			log.Warnf("Test run failed")
			log.Printf("isRetryOnFail=true - retrying...")
			return runTest(buildTestParams, outputTool, xcprettyOptions, false, false, swiftPackagesPath)
		}

		return fullOutputStr, exitCode, testError
	}

	// Clean output directory, otherwise after retry test run, xcodebuild fails with `error: Existing file at -resultBundlePath "..."`
	if err := os.RemoveAll(buildTestParams.TestOutputDir); err != nil {
		return "", 1, fmt.Errorf("failed to clean test output directory: %s, error: %s", buildTestParams.TestOutputDir, err)
	}
	buildParams := buildTestParams.BuildParams

	xcodebuildArgs := []string{buildParams.Action, buildParams.ProjectPath, "-scheme", buildParams.Scheme}
	if buildTestParams.CleanBuild {
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
	if buildTestParams.BuildBeforeTest {
		xcodebuildArgs = append(xcodebuildArgs, "build")
	}

	// Disable indexing during the build.
	// Indexing is needed for autocomplete, ability to quickly jump to definition, get class and method help by alt clicking.
	// Which are not needed in CI environment.
	if buildParams.DisableIndexWhileBuilding {
		xcodebuildArgs = append(xcodebuildArgs, "COMPILER_INDEX_STORE_ENABLE=NO")
	}

	xcodebuildArgs = append(xcodebuildArgs, "test", "-destination", buildParams.DeviceDestination)
	xcodebuildArgs = append(xcodebuildArgs, "-resultBundlePath", buildTestParams.TestOutputDir)

	if buildTestParams.GenerateCodeCoverage {
		xcodebuildArgs = append(xcodebuildArgs, "GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES")
		xcodebuildArgs = append(xcodebuildArgs, "GCC_GENERATE_TEST_COVERAGE_FILES=YES")
	}

	if buildTestParams.AdditionalOptions != "" {
		options, err := shellquote.Split(buildTestParams.AdditionalOptions)
		if err != nil {
			return "", 1, fmt.Errorf("failed to parse additional options (%s), error: %s", buildTestParams.AdditionalOptions, err)
		}
		xcodebuildArgs = append(xcodebuildArgs, options...)
	}

	xcprettyArgs := []string{}
	if xcprettyOptions != "" {
		options, err := shellquote.Split(xcprettyOptions)
		if err != nil {
			return "", 1, fmt.Errorf("failed to parse additional options (%s), error: %s", xcprettyOptions, err)
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
		xcprettyArgs = append(xcprettyArgs, options...)
	}

	log.Infof("Running the tests...")

	var rawOutput string
	var err error
	var exit int
	if outputTool == xcprettyTool {
		rawOutput, exit, err = runPrettyXcodeBuildCmd(true, xcprettyArgs, xcodebuildArgs)
	} else {
		rawOutput, exit, err = runXcodeBuildCmd(xcodebuildArgs...)
	}

	if err != nil {
		return handleTestError(rawOutput, exit, err)
	}

	return rawOutput, exit, nil
}
