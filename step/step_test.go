package step

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-io/go-xcode/v2/xcodeversion"
	"github.com/bitrise-steplib/steps-xcode-test/step/mocks"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type configParserMocks struct {
	deviceFinder *mocks.DeviceFinder
	pathModifier *mocks.PathModifier
}

type stepMocks struct {
	xcodeRunnerInstaller *mocks.RunnerDependencyInstaller
	xcodebuilder         *mocks.Xcodebuild
	simulatorManager     *mocks.SimulatorManager
	cache                *mocks.SwiftPackageCache
	outputExporter       *mocks.Exporter
	pathModifier         *mocks.PathModifier
	pathProvider         *mocks.PathProvider
}

func Test_GivenStep_WhenRuns_ThenXcodebuildGetsCalled(t *testing.T) {
	// Given
	step, mocks := createStepAndMocks(t)

	mocks.xcodebuilder.On("RunTest", mock.Anything).Return("", 0, nil)
	mocks.simulatorManager.On("ResetLaunchServices").Return(nil)
	mocks.cache.On("SwiftPackagesPath", mock.Anything).Return("", nil)
	mocks.pathProvider.On("CreateTempDir", mock.Anything).Return("tmp_dir", nil)

	config := Config{
		ProjectPath: "./project.xcodeproj",
		Scheme:      "Project",

		XcodeMajorVersion: 13,
		Simulator:         destination.Device{ID: "1234"},
		IsSimulatorBooted: true,

		TestRepetitionMode:            "none",
		MaximumTestRepetitions:        0,
		RelaunchTestForEachRepetition: true,
		RetryTestsOnFailure:           false,

		LogFormatter:       "xcodebuild",
		PerformCleanAction: false,

		CacheLevel: "",

		CollectSimulatorDiagnostics: never,
		HeadlessMode:                true,
	}

	// When
	_, err := step.Run(config)

	// Then
	require.NoError(t, err)
	mocks.xcodebuilder.AssertCalled(t, "RunTest", mock.Anything)
}

func Test_GivenXcode13OrNewer_WhenShouldRetryTestOnFailIsSet_ThenFails(t *testing.T) {
	// Given
	envValues := defaultEnvValues()
	envValues["should_retry_test_on_fail"] = "yes"

	configParser, _ := createConfigParser(t, envValues, newVersion(13))

	// When
	_, err := configParser.ProcessConfig()

	// Then
	require.Error(t, err)
}

func Test_GivenXcode12OrOlder_WhenTestRepetitionModeIsSet_ThenFails(t *testing.T) {
	// Given
	envValues := defaultEnvValues()
	envValues["test_repetition_mode"] = "retry_on_failure"

	ver := newVersion(12)
	configParser, _ := createConfigParser(t, envValues, ver)

	// When
	_, err := configParser.ProcessConfig()

	// Then
	require.Error(t, err)
}

func Test_GivenTestRepetitionModeIsNone_WhenRelaunchTestsForEachRepetitionIsSet_ThenFails(t *testing.T) {
	// Given
	envValues := defaultEnvValues()
	envValues["relaunch_tests_for_each_repetition"] = "yes"

	ver := newVersion(12)
	configParser, mocks := createConfigParser(t, envValues, ver)

	path := strings.TrimPrefix(envValues["project_path"], ".")
	mocks.pathModifier.On("AbsPath", mock.Anything).Return(path, nil)
	mocks.deviceFinder.On("FindDevice", mock.Anything, mock.Anything).Return(defaultSimulator(), nil)

	// When
	_, err := configParser.ProcessConfig()

	// Then
	require.Error(t, err)
}

func Test_GivenStep_WhenInstallXcpretty_ThenInstallIt(t *testing.T) {
	// Given
	step, mocks := createStepAndMocks(t)
	ver, err := version.NewVersion("1.0")
	if err != nil {
		assert.Fail(t, fmt.Sprintf("%s", err))
	}

	mocks.xcodeRunnerInstaller.On("CheckInstall", mock.Anything).Return(ver, nil)

	// When
	err = step.InstallDeps()

	// Then
	assert.NoError(t, err)
	mocks.xcodeRunnerInstaller.AssertExpectations(t)
}

func Test_GivenLogFormatterIsXcbeautify_WhenParsesConfig_ThenAdditionalOptionsWork(t *testing.T) {
	// Given
	envValues := defaultEnvValues()
	envValues["log_formatter"] = "xcbeautify"
	envValues["xcbeautify_options"] = "'--is-ci' '-q'"

	ver := newVersion(13)
	configParser, mocks := createConfigParser(t, envValues, ver)

	path := strings.TrimPrefix(envValues["project_path"], ".")
	mocks.pathModifier.On("AbsPath", mock.Anything).Return(path, nil)
	device := defaultSimulator()
	mocks.deviceFinder.On("FindDevice", mock.Anything, mock.Anything).Return(device, nil)

	// When
	actualConfig, err := configParser.ProcessConfig()

	// Then
	require.NoError(t, err)

	expectedConfig := Config{
		ProjectPath: "/_tmp/BullsEye.xcworkspace",
		Scheme:      "BullsEye",

		Simulator:         device,
		IsSimulatorBooted: false,

		XcodeMajorVersion: 13,

		TestRepetitionMode:            "none",
		MaximumTestRepetitions:        3,
		RelaunchTestForEachRepetition: false,
		RetryTestsOnFailure:           false,

		XcodebuildOptions: []string{},

		LogFormatter:        "xcbeautify",
		LogFormatterOptions: []string{"--is-ci", "-q"},
		PerformCleanAction:  false,

		CacheLevel: "swift_packages",

		CollectSimulatorDiagnostics: never,
		HeadlessMode:                true,
	}
	require.Equal(t, expectedConfig, actualConfig)
}

func Test_GivenStep_WhenExportsTestResult_ThenSetsCorrectly(t *testing.T) {
	tests := []struct {
		name       string
		testFailed bool
	}{
		{
			name:       "Exports success status",
			testFailed: false,
		},
		{
			name:       "Exports failure status",
			testFailed: true,
		},
	}

	for _, test := range tests {
		t.Log(test.name)

		runExportTest(t, test.testFailed)
	}
}

func runExportTest(t *testing.T, testFailed bool) {
	// Given
	step, mocks := createStepAndMocks(t)

	mocks.outputExporter.On("ExportTestRunResult", testFailed)

	// When
	err := step.Export(Result{}, testFailed)

	// Then
	assert.NoError(t, err)

	mocks.outputExporter.AssertCalled(t, "ExportTestRunResult", testFailed)
}

func Test_GivenStep_WhenExport_ThenExportsAllTestArtifacts(t *testing.T) {
	// Given
	step, mocks := createStepAndMocks(t)
	result := defaultResult()
	diagnosticsName := filepath.Base(result.SimulatorDiagnosticsPath)

	mocks.outputExporter.On("ExportTestRunResult", mock.Anything)
	mocks.outputExporter.On("ExportXCResultBundle", result.DeployDir, result.XcresultPath, result.Scheme)
	mocks.outputExporter.On("ExportXcodebuildBuildLog", result.DeployDir, result.XcodebuildBuildLog).Return(nil)
	mocks.outputExporter.On("ExportXcodebuildTestLog", result.DeployDir, result.XcodebuildTestLog).Return(nil)
	mocks.outputExporter.On("ExportSimulatorDiagnostics", result.DeployDir, result.SimulatorDiagnosticsPath, diagnosticsName).Return(nil)

	// When
	err := step.Export(result, false)

	// Then
	assert.NoError(t, err)

	mocks.outputExporter.AssertCalled(t, "ExportXCResultBundle", result.DeployDir, result.XcresultPath, result.Scheme)
	mocks.outputExporter.AssertCalled(t, "ExportXcodebuildBuildLog", result.DeployDir, result.XcodebuildBuildLog)
	mocks.outputExporter.AssertCalled(t, "ExportXcodebuildTestLog", result.DeployDir, result.XcodebuildTestLog)
	mocks.outputExporter.AssertCalled(t, "ExportSimulatorDiagnostics", result.DeployDir, result.SimulatorDiagnosticsPath, diagnosticsName)
}

// Helpers

func defaultEnvValues() map[string]string {
	return map[string]string{
		"project_path":                       "./_tmp/BullsEye.xcworkspace",
		"scheme":                             "BullsEye",
		"destination":                        "platform=iOS Simulator,name=iPhone 8 Plus,OS=latest",
		"test_repetition_mode":               "none",
		"maximum_test_repetitions":           "3",
		"relaunch_tests_for_each_repetition": "no",
		"should_retry_test_on_fail":          "no",
		"perform_clean_action":               "no",
		"log_formatter":                      "xcpretty",
		"cache_level":                        "swift_packages",
		"verbose_log":                        "no",
		"collect_simulator_diagnostics":      "never",
		"headless_mode":                      "yes",
	}
}

func defaultSimulator() destination.Device {
	return destination.Device{
		Name:   "iPhone 8 Plus",
		ID:     "E8C36A8B-543A-4477-BB91-699C0A9EA352",
		Status: "Shutdown",
	}
}

func defaultResult() Result {
	return Result{
		Scheme:                   "Scheme",
		DeployDir:                "DeployDir",
		XcresultPath:             "XcresultPath",
		XcodebuildBuildLog:       "XcodebuildBuildLog",
		XcodebuildTestLog:        "XcodebuildTestLog",
		SimulatorDiagnosticsPath: "/testpath/SimulatorDiagnosticsPath",
	}
}

func createConfigParser(t *testing.T, envValues map[string]string, xcodeVersion xcodeversion.Version) (XcodeTestConfigParser, configParserMocks) {
	envRepository := mocks.NewRepository(t)

	if envValues != nil {
		call := envRepository.On("Get", mock.Anything)
		call.RunFn = func(arguments mock.Arguments) {
			key := arguments[0].(string)
			value := envValues[key]
			call.ReturnArguments = mock.Arguments{value, nil}
		}
	}

	logger := log.NewLogger()
	inputParser := stepconf.NewInputParser(envRepository)
	deviceFinder := mocks.NewDeviceFinder(t)
	pathModifier := mocks.NewPathModifier(t)
	utils := NewUtils(logger)

	configParser := NewXcodeTestConfigParser(inputParser, logger, xcodeVersion, deviceFinder, pathModifier, utils)
	mocks := configParserMocks{
		deviceFinder: deviceFinder,
		pathModifier: pathModifier,
	}

	return configParser, mocks
}

func createStepAndMocks(t *testing.T) (XcodeTestRunner, stepMocks) {
	logger := log.NewLogger()
	xcodeRunnerInstaller := mocks.NewRunnerDependencyInstaller(t)
	xcodebuilder := mocks.NewXcodebuild(t)
	simulatorManager := mocks.NewSimulatorManager(t)
	cache := mocks.NewSwiftPackageCache(t)
	outputExporter := mocks.NewExporter(t)
	pathModifier := mocks.NewPathModifier(t)
	pathProvider := mocks.NewPathProvider(t)
	utils := NewUtils(logger)

	step := NewXcodeTestRunner(logger, xcodeRunnerInstaller, xcodebuilder, simulatorManager, cache, outputExporter, pathModifier, pathProvider, utils)
	mocks := stepMocks{
		xcodeRunnerInstaller: xcodeRunnerInstaller,
		xcodebuilder:         xcodebuilder,
		simulatorManager:     simulatorManager,
		cache:                cache,
		outputExporter:       outputExporter,
		pathModifier:         pathModifier,
		pathProvider:         pathProvider,
	}

	return step, mocks
}

func newVersion(major int64) xcodeversion.Version {
	return xcodeversion.Version{
		Version:      "test-version",
		BuildVersion: "test-build",
		MajorVersion: major,
	}
}
