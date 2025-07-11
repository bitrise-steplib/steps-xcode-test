package output

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/converters/xcresult3/model3"
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
	envRepository *mocks.Repository
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

func Test_GivenSimulatorDiagnostics_WhenExporting_ThenCopiesItAndSetsEnvVariable(t *testing.T) {
	// Given
	name := "Simulator"
	tempDir := t.TempDir()

	diagnosticsDir := filepath.Join(tempDir, "diagnostics")

	diagnosticsFile := filepath.Join(diagnosticsDir, "simulatorDiagnostics.txt")
	err := fileutil.NewFileManager().Write(diagnosticsFile, "test-diagnostics", 0777)

	require.NoError(t, err)
	require.FileExists(t, diagnosticsFile)

	exporter, _ := createSutAndMocks()

	// When
	err = exporter.ExportSimulatorDiagnostics(tempDir, diagnosticsDir, name)

	// Then
	assert.NoError(t, err)
	assert.True(t, isPathExists(filepath.Join(tempDir, name+".zip")))
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
	mocks.envRepository.AssertCalled(t, "Set", "BITRISE_FLAKY_TEST_CASES", "- BullsEyeFlakyTests.testFlakyFeature()\n")
}

func Test_ExportFlakyTestCases(t *testing.T) {
	commandFactory := new(commonMocks.CommandFactory)
	envRepository := new(mocks.Repository)
	envRepository.On("Set", mock.Anything, mock.Anything).Return(nil)

	exporter := exporter{
		envRepository:     envRepository,
		logger:            log.NewLogger(),
		outputExporter:    export.NewExporter(commandFactory),
		testAddonExporter: nil,
	}

	testSummary := model3.TestSummary{
		TestPlans: []model3.TestPlan{
			{
				TestBundles: []model3.TestBundle{
					{
						Name: "BullsEyeFlakyTests",
						TestSuites: []model3.TestSuite{
							{
								Name: "BullsEyeFlakyTests",
								TestCases: []model3.TestCaseWithRetries{
									{
										TestCase: model3.TestCase{
											Name:      "testFlakyFeature()",
											ClassName: "BullsEyeFlakyTests",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	exporter.collectFlakyTestPlans(testSummary)
}

func Test_exporter_collectFlakyTestPlans(t *testing.T) {

	tests := []struct {
		name         string
		testPlans    []model3.TestPlan
		wantEnvValue string
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
			wantEnvValue: "- TestSuite1.TestCase1\n- TestSuite1.TestCase2\n- TestSuite2.TestCase1\n",
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
			wantEnvValue: "- TestSuite1.TestCase1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRepository := new(mocks.Repository)
			envRepository.On("Set", mock.Anything, mock.Anything).Return(nil)

			exporter := exporter{
				envRepository:     envRepository,
				logger:            log.NewLogger(),
				outputExporter:    export.NewExporter(new(commonMocks.CommandFactory)),
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
		})
	}
}

// Helpers

func createSutAndMocks() (Exporter, testingMocks) {
	commandFactory := new(commonMocks.CommandFactory)
	envRepository := new(mocks.Repository)
	envRepository.On("Set", mock.Anything, mock.Anything).Return(nil)

	exporter := NewExporter(envRepository, log.NewLogger(), export.NewExporter(commandFactory), nil)

	return exporter, testingMocks{
		envRepository: envRepository,
	}
}

func isPathExists(path string) bool {
	isExist, _ := pathutil.NewPathChecker().IsPathExists(path)
	return isExist
}
