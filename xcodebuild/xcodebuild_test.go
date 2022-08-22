package xcodebuild

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild/mocks"
	"github.com/bitrise-steplib/steps-xcode-test/xcodecommand"
	"github.com/stretchr/testify/mock"
)

const xcconfigPath = "xcconfigPath"

type testingMocks struct {
	pathChecker        *mocks.PathChecker
	fileManager        *mocks.FileManager
	xcconfigWriter     *mocks.XcconfigWriter
	xcodeCommandRunner *mocks.XcodeCommandRunner
}

func Test_GivenXcodebuild_WhenInvoked_ThenUsesCorrectArguments(t *testing.T) {
	tests := []struct {
		name  string
		input func() TestRunParams
	}{
		{
			name: "Translates default parameters",
			input: func() TestRunParams {
				return runParameters()
			},
		},
		{
			name: "-run-tests-until-failure maps correctly",
			input: func() TestRunParams {
				parameters := runParameters()
				parameters.TestParams.TestRepetitionMode = "until_failure"
				parameters.TestParams.MaximumTestRepetitions = 3

				return parameters
			},
		},
		{
			name: "-retry-tests-on-failure maps correctly",
			input: func() TestRunParams {
				parameters := runParameters()
				parameters.TestParams.TestRepetitionMode = "retry_on_failure"
				parameters.TestParams.MaximumTestRepetitions = 11

				return parameters
			},
		},
		{
			name: "Disabling -test-repetition-relaunch-enabled maps correctly",
			input: func() TestRunParams {
				parameters := runParameters()
				parameters.TestParams.RelaunchTestsForEachRepetition = false

				return parameters
			},
		},
		{
			name: "Swift package",
			input: func() TestRunParams {
				parameters := runParameters()
				parameters.TestParams.ProjectPath = "MyPackage/Package.swift"

				return parameters
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runArgumentsTest(t, test.input())
		})
	}
}

func runArgumentsTest(t *testing.T, input TestRunParams) {
	// Given
	xcodebuild, mocks := createXcodebuildAndMocks(t)

	arguments := argumentsFromRunParameters(input)
	mocks.xcodeCommandRunner.On("Run", mock.Anything, arguments, []string(nil)).
		Return(xcodecommand.Output{}, nil)

	// When
	_, _, _ = xcodebuild.RunTest(input)

	// Then
	mocks.xcodeCommandRunner.AssertExpectations(t)
}

func Test_GivenTestRunError_WhenSwiftPackageError_ThenRetries(t *testing.T) {
	// Given
	const swiftPMErrMsg = "Could not resolve package dependencies:"
	parameters := runParameters()
	xcodebuild, mocks := createXcodebuildAndMocks(t)

	mocks.xcodeCommandRunner.On("Run", ".", mock.Anything, mock.Anything).
		Return(xcodecommand.Output{
			ExitCode: 1,
			RawOut:   []byte(swiftPMErrMsg),
		}, errors.New("some error"))

	mocks.fileManager.On("RemoveAll", parameters.SwiftPackagesPath).Return(nil)
	mocks.fileManager.On("RemoveAll", parameters.TestParams.TestOutputDir).Return(nil)

	// When
	_, _, _ = xcodebuild.RunTest(parameters)

	// Then
	mocks.xcodeCommandRunner.AssertNumberOfCalls(t, "Run", 2)
	mocks.fileManager.AssertNumberOfCalls(t, "RemoveAll", 2)
}

func Test_GivenTestRunError_WhenOneOfTheNamedErrorsHappened_ThenActsBasedOnTheConfig(t *testing.T) {
	errors := errorsToBeRetried()
	tests := []struct {
		name          string
		errors        []string
		numberOfCalls int
		parameters    func() TestRunParams
	}{
		{
			name:          "Reruns tests when test runner error retry is enabled",
			errors:        errors,
			numberOfCalls: 2,
			parameters: func() TestRunParams {
				return runParameters()
			},
		},
		{
			name:          "Does nothing when test runner error retry is disabled",
			errors:        errors,
			numberOfCalls: 1,
			parameters: func() TestRunParams {
				parameters := runParameters()
				parameters.RetryOnTestRunnerError = false

				return parameters
			},
		},
	}

	for _, test := range tests {
		t.Logf(test.name)

		for _, errorString := range test.errors {
			t.Logf("Testing: %s", errorString)

			runRunnerErrorTests(t, test.numberOfCalls, test.parameters(), errorString)
		}
	}
}

func Test_GivenTestRunError_WhenAnUnknownErrorHappened_ThenActsBasedOnTheConfig(t *testing.T) {
	const xcodeOutput = "unknown error: we are definitely not prepared for this"

	tests := []struct {
		name          string
		numberOfCalls int
		parameters    func() TestRunParams
	}{
		{
			name:          "Reruns tests when should_retry_test_on_fail is enabled",
			numberOfCalls: 2,
			parameters: func() TestRunParams {
				parameters := runParameters()
				parameters.TestParams.RetryTestsOnFailure = true

				return parameters
			},
		},
		{
			name:          "Does nothing when should_retry_test_on_fail is disabled",
			numberOfCalls: 1,
			parameters: func() TestRunParams {
				return runParameters()
			},
		},
	}

	for _, test := range tests {
		t.Logf(test.name)

		runRunnerErrorTests(t, test.numberOfCalls, test.parameters(), xcodeOutput)
	}
}

func runRunnerErrorTests(t *testing.T, expectedNumberOfCreateCalls int, parameters TestRunParams, xcodeOutput string) {
	// Given
	xcodebuild, mocks := createXcodebuildAndMocks(t)

	mocks.xcodeCommandRunner.On("Run", ".", mock.Anything, mock.Anything).
		Return(xcodecommand.Output{
			ExitCode: 1,
			RawOut:   []byte(xcodeOutput),
		}, errors.New("some error"))
	mocks.fileManager.On("RemoveAll", parameters.TestParams.TestOutputDir).Return(nil)

	// When
	_, _, _ = xcodebuild.RunTest(parameters)

	// Then
	mocks.xcodeCommandRunner.AssertNumberOfCalls(t, "Run", expectedNumberOfCreateCalls)
	mocks.xcodeCommandRunner.AssertExpectations(t)
}

func Test_GivenXcprettyFormatter_WhenEnabled_ThenUsesCorrectArguments(t *testing.T) {
	// Given
	outputPath := "path/to/output"

	parameters := runParameters()
	parameters.LogFormatter = "xcpretty"
	parameters.XcprettyOptions = fmt.Sprintf("--color --report html --output %s", outputPath)

	xcodebuild, mocks := createXcodebuildAndMocks(t)

	mocks.pathChecker.On("IsPathExists", outputPath).Return(true, nil)
	mocks.fileManager.On("Remove", outputPath).Return(nil)

	mocks.xcodeCommandRunner.On("Run", ".", mock.Anything, strings.Fields(parameters.XcprettyOptions)).
		Return(xcodecommand.Output{}, nil)

	// When
	_, _, _ = xcodebuild.RunTest(parameters)

	// Then
	mocks.xcodeCommandRunner.AssertExpectations(t)
	mocks.pathChecker.AssertExpectations(t)
	mocks.fileManager.AssertExpectations(t)
}

// Helpers

func createXcodebuildAndMocks(t *testing.T) (Xcodebuild, testingMocks) {
	logger := log.NewLogger()
	pathChecker := new(mocks.PathChecker)
	fileManager := new(mocks.FileManager)
	xcconfigWriter := new(mocks.XcconfigWriter)
	xcodeCommandRunner := mocks.NewXcodeCommandRunner(t)

	xcconfigWriter.On("Write", mock.Anything).Return(xcconfigPath, nil)

	xcodebuild := NewXcodebuild(logger, pathChecker, fileManager, xcconfigWriter, xcodeCommandRunner)

	return xcodebuild, testingMocks{
		pathChecker:        pathChecker,
		fileManager:        fileManager,
		xcconfigWriter:     xcconfigWriter,
		xcodeCommandRunner: xcodeCommandRunner,
	}
}

func runParameters() TestRunParams {
	testParams := TestParams{
		ProjectPath:                    "ProjectPath.xcodeproj",
		Scheme:                         "Scheme",
		Destination:                    "Destination",
		TestPlan:                       "TestPlan",
		TestOutputDir:                  "TestOutputDir",
		TestRepetitionMode:             "none",
		MaximumTestRepetitions:         3,
		RelaunchTestsForEachRepetition: true,
		XCConfigContent:                "XCConfigContent",
		PerformCleanAction:             false,
		RetryTestsOnFailure:            false,
		AdditionalOptions:              []string{"AdditionalOptions"},
	}

	return TestRunParams{
		TestParams:                         testParams,
		LogFormatter:                       "xcodebuild",
		XcprettyOptions:                    "",
		RetryOnTestRunnerError:             true,
		RetryOnSwiftPackageResolutionError: true,
		SwiftPackagesPath:                  "SwiftPackagesPath",
		XcodeMajorVersion:                  13,
	}
}

func argumentsFromRunParameters(parameters TestRunParams) []string {
	var arguments []string

	if !strings.HasSuffix(parameters.TestParams.ProjectPath, "Package.swift") {
		arguments = append(arguments, "-project", parameters.TestParams.ProjectPath)
	}

	arguments = append(arguments, "-scheme", parameters.TestParams.Scheme)

	if parameters.TestParams.PerformCleanAction {
		arguments = append(arguments, "clean")
	}

	arguments = append(arguments, "test", "-destination", parameters.TestParams.Destination)

	if parameters.TestParams.TestPlan != "" {
		arguments = append(arguments, "-testPlan", parameters.TestParams.TestPlan)
	}

	arguments = append(arguments, "-resultBundlePath", parameters.TestParams.TestOutputDir)

	switch parameters.TestParams.TestRepetitionMode {
	case TestRepetitionUntilFailure:
		arguments = append(arguments, "-run-tests-until-failure")
	case TestRepetitionRetryOnFailure:
		arguments = append(arguments, "-retry-tests-on-failure")
	}

	if parameters.TestParams.TestRepetitionMode != TestRepetitionNone {
		arguments = append(arguments, "-test-iterations", strconv.Itoa(parameters.TestParams.MaximumTestRepetitions))
	}

	if parameters.TestParams.RelaunchTestsForEachRepetition {
		arguments = append(arguments, "-test-repetition-relaunch-enabled", "YES")
	}

	if parameters.TestParams.XCConfigContent != "" {
		arguments = append(arguments, "-xcconfig", xcconfigPath)
	}

	arguments = append(arguments, parameters.TestParams.AdditionalOptions...)

	return arguments
}

func errorsToBeRetried() []string {
	return []string{
		"iPhoneSimulator: Timed out waiting",
		"Terminating app due to uncaught exception '_XCTestCaseInterruptionException'",
		"Early unexpected exit, operation never finished bootstrapping - no restart will be attempted",
		"Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl:",
		`Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner.`,
		`App state is still not running active, state = XCApplicationStateNotRunning`,
		`UI Testing Failure - App accessibility isn't loaded`,
		`Test runner failed to initialize for UI testing`,
		`Timed out registering for testing event accessibility notifications`,
		`Test runner never began executing tests after launching.`,
	}
}
