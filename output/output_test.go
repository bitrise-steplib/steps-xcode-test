package output

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-utils/v2/ziputil"
	"github.com/bitrise-io/go-xcode/v2/testresult/xcresult3/model3"
	commonMocks "github.com/bitrise-steplib/steps-xcode-test/mocks"
	"github.com/bitrise-steplib/steps-xcode-test/output/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	xcodeTestResultKey     = "BITRISE_XCODE_TEST_RESULT"
	xcodebuildBuildLogPath = "BITRISE_XCODEBUILD_BUILD_LOG_PATH"
	xcodebuildTestLogPath  = "BITRISE_XCODEBUILD_TEST_LOG_PATH"
)

type testingMocks struct {
	envRepository  *mocks.Repository
	commandFactory *commonMocks.CommandFactory
}

func Test_GivenSuccessfulTest_WhenExportingTestRunResults_ThenSetsEnvVariableToSuccess(t *testing.T) {
	// Given
	exporter, mocks := createSutAndMocks()

	// When
	exporter.ExportTestRunResult(false)

	// Then
	mocks.envRepository.AssertCalled(t, "Set", xcodeTestResultKey, "succeeded")
}

func Test_GivenFailedTest_WhenExportingTestRunResults_ThenSetsEnvVariableToFailure(t *testing.T) {
	// Given
	exporter, mocks := createSutAndMocks()

	// When
	exporter.ExportTestRunResult(true)

	// Then
	mocks.envRepository.AssertCalled(t, "Set", xcodeTestResultKey, "failed")
}

func Test_GivenBuildLog_WhenExporting_ThenCopiesItAndSetsEnvVariable(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "xcodebuild_build.log")

	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	exporter, mocks := createSutAndMocks()

	// When
	err := exporter.ExportXcodebuildBuildLog(tempDir, "xcodebuild build log")

	// Then
	mocks.envRepository.AssertCalled(t, "Set", xcodebuildBuildLogPath, logPath)

	assert.NoError(t, err)
	assert.True(t, isPathExists(logPath))
}

func Test_GivenTestLog_WhenExporting_ThenCopiesItAndSetsEnvVariable(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "xcodebuild_test.log")

	exporter, mocks := createSutAndMocks()

	// When
	err := exporter.ExportXcodebuildTestLog(tempDir, "xcodebuild test log")

	// Then
	mocks.envRepository.AssertCalled(t, "Set", xcodebuildTestLogPath, logPath)

	assert.NoError(t, err)
	assert.True(t, isPathExists(logPath))
}

func Test_GivenFlakyTestCases_WhenExporting_ThenSetsEnvVariable(t *testing.T) {
	// Given
	_, b, _, _ := runtime.Caller(0)
	outputPackageDir := filepath.Dir(b)
	testDataDir := filepath.Join(outputPackageDir, "testdata")
	xcresultPath := filepath.Join(testDataDir, "xcresult3-flaky-with-rerun.xcresult")

	exporter, mocks := createSutAndMocks()

	// When
	err := exporter.ExportFlakyTestCases(xcresultPath, false)

	// Then
	assert.NoError(t, err)
	mocks.envRepository.AssertCalled(t, "Set", "BITRISE_FLAKY_TEST_CASES", "- BullsEyeFlakyTests.BullsEyeFlakyTests.testFlakyFeature()\n")
}

func Test_exporter_collectAndExportFlakyTestPlans(t *testing.T) {
	longTestCaseName := strings.Repeat("a", flakyTestCasesEnvVarSizeLimitInBytes)

	tests := []struct {
		name         string
		testPlans    []model3.TestPlan
		wantEnvValue string
		wantLogArgs  []interface{}
	}{
		{
			name: "No flaky tests",
			testPlans: []model3.TestPlan{{TestBundles: []model3.TestBundle{{Name: "TestBundle", TestSuites: []model3.TestSuite{
				{
					Name: "TestSuite",
					TestCases: []model3.TestCaseWithRetries{
						{
							TestCase: model3.TestCase{Name: "TestCase", ClassName: "TestClass"},
						},
					},
				},
			}}}}},
			wantEnvValue: "",
		},
		{
			name: "Multiple flaky tests",
			testPlans: []model3.TestPlan{{TestBundles: []model3.TestBundle{{Name: "TestBundle", TestSuites: []model3.TestSuite{
				{
					Name: "TestSuite1",
					TestCases: []model3.TestCaseWithRetries{
						{
							TestCase: model3.TestCase{Name: "TestCase1", ClassName: "TestSuite1", Result: model3.TestResultPassed},
							Retries: []model3.TestCase{
								{Name: "TestCase1_Retry1", ClassName: "TestSuite1", Result: model3.TestResultFailed},
								{Name: "TestCase1_Retry2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
							},
						},
						{
							TestCase: model3.TestCase{Name: "TestCase2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
							Retries: []model3.TestCase{
								{Name: "TestCase2_Retry1", ClassName: "TestSuite1", Result: model3.TestResultFailed},
								{Name: "TestCase2_Retry2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
							},
						},
						{
							TestCase: model3.TestCase{Name: "TestCase3", ClassName: "TestSuite1", Result: model3.TestResultPassed},
						},
					},
				},
				{
					Name: "TestSuite2",
					TestCases: []model3.TestCaseWithRetries{
						{
							TestCase: model3.TestCase{Name: "TestCase1", ClassName: "TestSuite2", Result: model3.TestResultPassed},
							Retries: []model3.TestCase{
								{Name: "TestCase1_Retry1", ClassName: "TestSuite1", Result: model3.TestResultFailed},
								{Name: "TestCase1_Retry2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
							},
						},
					},
				},
			}}}}},
			wantEnvValue: "- TestBundle.TestSuite1.TestCase1\n- TestBundle.TestSuite1.TestCase2\n- TestBundle.TestSuite2.TestCase1\n",
		},
		{
			name: "Same flaky test exported once",
			testPlans: []model3.TestPlan{{TestBundles: []model3.TestBundle{
				{
					Name: "TestBundle1", TestSuites: []model3.TestSuite{
						{
							Name: "TestSuite1",
							TestCases: []model3.TestCaseWithRetries{
								{
									TestCase: model3.TestCase{Name: "TestCase1", ClassName: "TestSuite1", Result: model3.TestResultPassed},
									Retries: []model3.TestCase{
										{Name: "TestCase1_Retry1", ClassName: "TestSuite1", Result: model3.TestResultFailed},
										{Name: "TestCase1_Retry2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
									},
								},
							},
						},
					},
				},
				{
					Name: "TestBundle2", TestSuites: []model3.TestSuite{
						{
							Name: "TestSuite1",
							TestCases: []model3.TestCaseWithRetries{
								{
									TestCase: model3.TestCase{Name: "TestCase1", ClassName: "TestSuite1", Result: model3.TestResultPassed},
									Retries: []model3.TestCase{
										{Name: "TestCase1_Retry1", ClassName: "TestSuite1", Result: model3.TestResultFailed},
										{Name: "TestCase1_Retry2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
									},
								},
							},
						},
					},
				},
			}}},
			wantEnvValue: "- TestBundle1.TestSuite1.TestCase1\n- TestBundle2.TestSuite1.TestCase1\n",
		},
		{
			name: "Flaky test cases env var size is limited",
			testPlans: []model3.TestPlan{{TestBundles: []model3.TestBundle{
				{
					Name: "TestBundle1", TestSuites: []model3.TestSuite{
						{
							Name: "TestSuite1",
							TestCases: []model3.TestCaseWithRetries{
								{
									TestCase: model3.TestCase{Name: "TestCase1", ClassName: "TestSuite1", Result: model3.TestResultPassed},
									Retries: []model3.TestCase{
										{Name: "TestCase1_Retry1", ClassName: "TestSuite1", Result: model3.TestResultFailed},
										{Name: "TestCase1_Retry2", ClassName: "TestSuite1", Result: model3.TestResultPassed},
									},
								},
							},
						},
					},
				},
				{
					Name: "TestBundle2", TestSuites: []model3.TestSuite{
						{
							Name: "TestSuite1",
							TestCases: []model3.TestCaseWithRetries{
								{
									TestCase: model3.TestCase{Name: longTestCaseName, ClassName: "TestSuite1", Result: model3.TestResultPassed},
									Retries: []model3.TestCase{
										{Name: longTestCaseName, ClassName: "TestSuite1", Result: model3.TestResultFailed},
										{Name: longTestCaseName, ClassName: "TestSuite1", Result: model3.TestResultPassed},
									},
								},
							},
						},
					},
				},
			}}},
			wantEnvValue: "- TestBundle1.TestSuite1.TestCase1\n",
			wantLogArgs:  []interface{}{"%s env var size limit (%d characters) exceeded. Skipping %d test cases.", "BITRISE_FLAKY_TEST_CASES", flakyTestCasesEnvVarSizeLimitInBytes, 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRepository := new(mocks.Repository)
			envRepository.On("Set", mock.Anything, mock.Anything).Return(nil)

			logger := new(mocks.Logger)
			logger.On("Warnf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			exporter := exporter{
				envRepository:     envRepository,
				logger:            logger,
				outputExporter:    export.NewExporter(new(commonMocks.CommandFactory), new(commonMocks.FileManager), ziputil.NewZipManager(pathutil.NewPathChecker())),
				testAddonExporter: nil,
			}

			testSummary := model3.TestSummary{
				TestPlans: tt.testPlans,
			}

			flakyTestCases := exporter.collectFlakyTestPlans(testSummary)
			err := exporter.exportFlakyTestCases(flakyTestCases)
			require.NoError(t, err)

			if tt.wantEnvValue != "" {
				envRepository.AssertCalled(t, "Set", "BITRISE_FLAKY_TEST_CASES", tt.wantEnvValue)
			} else {
				envRepository.AssertNumberOfCalls(t, "Set", 0)
			}

			if len(tt.wantLogArgs) > 0 {
				logger.AssertCalled(t, "Warnf", tt.wantLogArgs...)
			} else {
				logger.AssertNumberOfCalls(t, "Warnf", 0)
			}
		})
	}
}

// Helpers

func Test_GivenXcresultDiagnostics_WhenExporting_ThenZipsThemIntoTheDeployDir(t *testing.T) {
	// Given
	deployDir := t.TempDir()
	exporter, mocks := createSutAndMocks()
	cmd := new(commonMocks.Command)
	cmd.On("PrintableCommandArgs").Return("xcrun xcresulttool export diagnostics")
	cmd.On("RunAndReturnTrimmedCombinedOutput").Return("Exported 1 object", nil)
	mocks.commandFactory.On("Create", "xcrun", mock.MatchedBy(isExportDiagnosticsArgs), mock.Anything).Run(func(args mock.Arguments) {
		// xcresulttool writes the bundle into --output-path
		bundleDir := filepath.Join(args.Get(1).([]string)[6], "0_Test_iPhone 17_Diagnostics", "simctl_diagnostics")
		require.NoError(t, os.MkdirAll(bundleDir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "diagnose.log"), []byte("diagnostics"), 0600))
	}).Return(cmd)

	// When
	err := exporter.ExportXcresultDiagnostics(deployDir, fakeXcresult(t), "Scheme")

	// Then
	require.NoError(t, err)
	assert.True(t, isPathExists(filepath.Join(deployDir, "Test-Scheme-diagnostics.zip")))
}

func Test_GivenXcresultWithoutSimulatorDiagnostics_WhenExporting_ThenExportsTheSessionLogs(t *testing.T) {
	// Given
	deployDir := t.TempDir()
	exporter, mocks := createSutAndMocks()
	cmd := new(commonMocks.Command)
	cmd.On("PrintableCommandArgs").Return("xcrun xcresulttool export diagnostics")
	cmd.On("RunAndReturnTrimmedCombinedOutput").Return("Exported 1 object", nil)
	mocks.commandFactory.On("Create", "xcrun", mock.MatchedBy(isExportDiagnosticsArgs), mock.Anything).Run(func(args mock.Arguments) {
		// only the test session logs, no simctl diagnose bundle
		sessionDir := filepath.Join(args.Get(1).([]string)[6], "0_Test_iPhone 17_Diagnostics", "Tests-Configuration")
		require.NoError(t, os.MkdirAll(sessionDir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "scheduling.log"), []byte("log"), 0600))
	}).Return(cmd)

	// When
	err := exporter.ExportXcresultDiagnostics(deployDir, fakeXcresult(t), "Scheme")

	// Then
	require.NoError(t, err)
	assert.True(t, isPathExists(filepath.Join(deployDir, "Test-Scheme-diagnostics.zip")))
}

func Test_GivenXcresultWithoutDiagnostics_WhenExporting_ThenReturnsErrNoDiagnostics(t *testing.T) {
	// Given
	deployDir := t.TempDir()
	exporter, mocks := createSutAndMocks()
	cmd := new(commonMocks.Command)
	cmd.On("PrintableCommandArgs").Return("xcrun xcresulttool export diagnostics")
	// xcodebuild did not reach the test phase: xcresulttool succeeds but exports nothing
	cmd.On("RunAndReturnTrimmedCombinedOutput").Return("Exported 0 objects", nil)
	mocks.commandFactory.On("Create", "xcrun", mock.MatchedBy(isExportDiagnosticsArgs), mock.Anything).Return(cmd)

	// When
	err := exporter.ExportXcresultDiagnostics(deployDir, fakeXcresult(t), "Scheme")

	// Then
	require.ErrorIs(t, err, ErrNoDiagnostics)
	assert.False(t, isPathExists(filepath.Join(deployDir, "Test-Scheme-diagnostics.zip")))
}

func Test_GivenXcresultDiagnostics_WhenExportFails_ThenReturnsError(t *testing.T) {
	// Given
	deployDir := t.TempDir()
	exporter, mocks := createSutAndMocks()
	cmd := new(commonMocks.Command)
	cmd.On("PrintableCommandArgs").Return("xcrun xcresulttool export diagnostics")
	cmd.On("RunAndReturnTrimmedCombinedOutput").Return("error: no diagnostics", fmt.Errorf("exit status 1"))
	mocks.commandFactory.On("Create", "xcrun", mock.MatchedBy(isExportDiagnosticsArgs), mock.Anything).Return(cmd)

	// When
	err := exporter.ExportXcresultDiagnostics(deployDir, fakeXcresult(t), "Scheme")

	// Then
	require.ErrorContains(t, err, "no diagnostics")
	assert.False(t, isPathExists(filepath.Join(deployDir, "Test-Scheme-diagnostics.zip")))
}

func Test_GivenNoXcresult_WhenExportingDiagnostics_ThenReturnsErrNoDiagnostics(t *testing.T) {
	// Given: xcodebuild never wrote the result bundle; xcresulttool must not even be called
	deployDir := t.TempDir()
	exporter, _ := createSutAndMocks()

	// When
	err := exporter.ExportXcresultDiagnostics(deployDir, filepath.Join(t.TempDir(), "Test-Scheme.xcresult"), "Scheme")

	// Then
	require.ErrorIs(t, err, ErrNoDiagnostics)
	assert.False(t, isPathExists(filepath.Join(deployDir, "Test-Scheme-diagnostics.zip")))
}

// fakeXcresult creates an empty result bundle directory and returns its path.
func fakeXcresult(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Test-Scheme.xcresult")
	require.NoError(t, os.Mkdir(path, 0700))
	return path
}

func isExportDiagnosticsArgs(args []string) bool {
	return len(args) == 7 && args[0] == "xcresulttool" && args[1] == "export" && args[2] == "diagnostics" &&
		args[3] == "--path" && strings.HasSuffix(args[4], "/Test-Scheme.xcresult") && args[5] == "--output-path"
}

func createSutAndMocks() (Exporter, testingMocks) {
	commandFactory := new(commonMocks.CommandFactory)
	fileManager := new(commonMocks.FileManager)
	envRepository := new(mocks.Repository)
	envRepository.On("Set", mock.Anything, mock.Anything).Return(nil)

	zipManager := ziputil.NewZipManager(pathutil.NewPathChecker())
	exporter := NewExporter(envRepository, log.NewLogger(), export.NewExporter(commandFactory, fileManager, zipManager), nil, commandFactory, zipManager, pathutil.NewPathProvider(), fileutil.NewFileManager())

	return exporter, testingMocks{
		envRepository:  envRepository,
		commandFactory: commandFactory,
	}
}

func isPathExists(path string) bool {
	isExist, _ := pathutil.NewPathChecker().IsPathExists(path)
	return isExist
}
