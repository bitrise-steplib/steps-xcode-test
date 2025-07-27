package xcodebuild

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/xcodecommand"
	commonMocks "github.com/bitrise-steplib/steps-xcode-test/mocks"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild/mocks"
	"github.com/stretchr/testify/mock"
)

const xcconfigPath = "xcconfigPath"

type testingMocks struct {
	fileManager        *mocks.FileManager
	xcconfigWriter     *mocks.XcconfigWriter
	xcodeCommandRunner *commonMocks.XcodeCommandRunner
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
	mocks.xcodeCommandRunner.On("Run", mock.Anything, arguments, []string{}).
		Return(xcodecommand.Output{}, nil)

	// When
	_, _, _ = xcodebuild.RunTest(input)

	// Then
	mocks.xcodeCommandRunner.AssertExpectations(t)
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
		t.Log(test.name)

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
			name:          "Does nothing when should_retry_test_on_fail is disabled",
			numberOfCalls: 1,
			parameters: func() TestRunParams {
				return runParameters()
			},
		},
	}

	for _, test := range tests {
		t.Log(test.name)

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
	parameters.LogFormatterOptions = []string{"--color", "--report", "html", "--output", outputPath}

	xcodebuild, mocks := createXcodebuildAndMocks(t)

	mocks.xcodeCommandRunner.On("Run", ".", mock.Anything, parameters.LogFormatterOptions).
		Return(xcodecommand.Output{}, nil)

	// When
	_, _, _ = xcodebuild.RunTest(parameters)

	// Then
	mocks.xcodeCommandRunner.AssertExpectations(t)
}

// Helpers

func createXcodebuildAndMocks(t *testing.T) (Xcodebuild, testingMocks) {
	logger := log.NewLogger()
	fileManager := new(mocks.FileManager)
	xcconfigWriter := new(mocks.XcconfigWriter)
	xcodeCommandRunner := commonMocks.NewXcodeCommandRunner(t)

	xcconfigWriter.On("Write", mock.Anything).Return(xcconfigPath, nil)

	xcodebuild := NewXcodebuild(logger, fileManager, xcconfigWriter, xcodeCommandRunner)

	return xcodebuild, testingMocks{
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
		AdditionalOptions:              []string{"AdditionalOptions"},
	}

	return TestRunParams{
		TestParams:                         testParams,
		LogFormatterOptions:                []string{},
		RetryOnTestRunnerError:             true,
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
