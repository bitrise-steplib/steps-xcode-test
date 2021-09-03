package step

import (
	"testing"

	mocklog "github.com/bitrise-io/go-utils/log/mocks"
	mockcache "github.com/bitrise-steplib/steps-xcode-test/cache/mocks"
	mocksimulator "github.com/bitrise-steplib/steps-xcode-test/simulator/mocks"
	mockxcodebuild "github.com/bitrise-steplib/steps-xcode-test/xcodebuild/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_WhenTestRuns_ThenXcodebuildGetsCalled(t *testing.T) {
	// Given
	logger := createLogger()

	xcodebuilder := new(mockxcodebuild.Xcodebuild)
	xcodebuilder.On("RunTest", mock.Anything).Return("", 0, nil)

	simulator := new(mocksimulator.Simulator)
	simulator.On("ResetLaunchServices").Return(nil)

	cache := new(mockcache.Cache)
	cache.On("SwiftPackagesPath", mock.Anything).Return("", nil)

	step := NewXcodeTestRunner(nil, logger, nil, xcodebuilder, simulator, cache, nil, nil)

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
