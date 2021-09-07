package step

import (
	"fmt"
	"testing"

	mocklog "github.com/bitrise-io/go-utils/log/mocks"
	mockPathutil "github.com/bitrise-io/go-utils/pathutil/mocks"
	mockcache "github.com/bitrise-steplib/steps-xcode-test/cache/mocks"
	mocksimulator "github.com/bitrise-steplib/steps-xcode-test/simulator/mocks"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	mockxcodebuild "github.com/bitrise-steplib/steps-xcode-test/xcodebuild/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_GivenIsSingleBuildFalse_WhenIsCleanBuildSet_ThenCleanCalledDuringBuild(t *testing.T) {
	// Given
	logger := createLogger()

	xcodebuilder := new(mockxcodebuild.Xcodebuild)
	xcodebuilder.On("RunBuild", mock.Anything, mock.Anything).Return("", 0, nil)
	xcodebuilder.On("RunTest", mock.Anything).Return("", 0, nil)

	simulatorManager := new(mocksimulator.Manager)
	simulatorManager.On("ResetLaunchServices").Return(nil)

	cache := new(mockcache.Cache)
	cache.On("SwiftPackagesPath", mock.Anything).Return("", nil)

	pathProvider := new(mockPathutil.PathProvider)
	pathProvider.On("CreateTempDir", mock.Anything).Return("tmp_dir", nil)

	step := NewXcodeTestRunner(nil, logger, nil, xcodebuilder, simulatorManager, cache, nil, nil, pathProvider)

	config := Config{
		ProjectPath: "./project.xcodeproj",
		Scheme:      "Project",

		XcodeMajorVersion: 13,
		SimulatorID:       "1234",
		IsSimulatorBooted: true,

		TestRepetitionMode:            "none",
		MaximumTestRepetitions:        0,
		RelaunchTestForEachRepetition: false,

		OutputTool:         "xcodebuild",
		IsSingleBuild:      false,
		IsCleanBuild:       true,
		BuildBeforeTesting: false,

		RetryTestsOnFailure:       false,
		DisableIndexWhileBuilding: false,
		GenerateCodeCoverageFiles: false,
		HeadlessMode:              true,

		SimulatorDebug: never,

		CacheLevel: "",
	}

	// When
	_, err := step.Run(config)

	// Then
	require.NoError(t, err)

	givenBuildParamsWithCleanBuild := xcodebuild.Params{
		Action:            "-project",
		ProjectPath:       "./project.xcodeproj",
		Scheme:            "Project",
		DeviceDestination: fmt.Sprintf("id=%s", "1234"),
		CleanBuild:        true,
	}
	xcodebuilder.AssertCalled(t, "RunBuild", givenBuildParamsWithCleanBuild, "xcodebuild")

	givenTestRunParamsWithoutCleanBuild := xcodebuild.TestRunParams{
		BuildTestParams: xcodebuild.TestParams{
			BuildParams:        givenBuildParamsWithCleanBuild,
			TestRepetitionMode: "none",
			TestOutputDir:      "tmp_dir/Test.xcresult",
			CleanBuild:         false,
		},
		OutputTool:                         "xcodebuild",
		XcodeMajorVersion:                  13,
		RetryOnTestRunnerError:             true,
		RetryOnSwiftPackageResolutionError: true,
	}
	xcodebuilder.AssertCalled(t, "RunTest", givenTestRunParamsWithoutCleanBuild)
}

func Test_WhenTestRuns_ThenXcodebuildGetsCalled(t *testing.T) {
	// Given
	logger := createLogger()

	xcodebuilder := new(mockxcodebuild.Xcodebuild)
	xcodebuilder.On("RunTest", mock.Anything).Return("", 0, nil)

	simulatorManager := new(mocksimulator.Manager)
	simulatorManager.On("ResetLaunchServices").Return(nil)

	cache := new(mockcache.Cache)
	cache.On("SwiftPackagesPath", mock.Anything).Return("", nil)

	pathProvider := new(mockPathutil.PathProvider)
	pathProvider.On("CreateTempDir", mock.Anything).Return("tmp_dir", nil)

	step := NewXcodeTestRunner(nil, logger, nil, xcodebuilder, simulatorManager, cache, nil, nil, pathProvider)

	config := Config{
		ProjectPath: "./project.xcodeproj",
		Scheme:      "Project",

		XcodeMajorVersion: 13,
		SimulatorID:       "1234",
		IsSimulatorBooted: true,

		TestRepetitionMode:            "none",
		MaximumTestRepetitions:        0,
		RelaunchTestForEachRepetition: true,

		OutputTool:         "xcodebuild",
		IsCleanBuild:       false,
		IsSingleBuild:      true,
		BuildBeforeTesting: false,

		RetryTestsOnFailure:       false,
		DisableIndexWhileBuilding: true,
		GenerateCodeCoverageFiles: false,
		HeadlessMode:              true,

		SimulatorDebug: never,

		CacheLevel: "",
	}

	// When
	_, err := step.Run(config)

	// Then
	require.NoError(t, err)
	xcodebuilder.AssertCalled(t, "RunTest", mock.Anything)
}

func createLogger() (logger *mocklog.Logger) {
	logger = new(mocklog.Logger)
	logger.On("Infof", mock.Anything, mock.Anything).Return()
	logger.On("Debugf", mock.Anything, mock.Anything).Return()
	logger.On("Donef", mock.Anything, mock.Anything).Return()
	logger.On("Printf", mock.Anything, mock.Anything).Return()
	logger.On("Errorf", mock.Anything, mock.Anything).Return()
	logger.On("Println").Return()
	logger.On("EnableDebugLog", mock.Anything).Return()
	return
}
