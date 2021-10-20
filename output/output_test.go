package output

import (
	mockenv "github.com/bitrise-io/go-utils/env/mocks"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path/filepath"
	"testing"
)

const (
	xcodeTestResultKey     = "BITRISE_XCODE_TEST_RESULT"
	xcodebuildBuildLogPath = "BITRISE_XCODEBUILD_BUILD_LOG_PATH"
	xcodebuildTestLogPath  = "BITRISE_XCODEBUILD_TEST_LOG_PATH"
)

type testingMocks struct {
	envRepository *mockenv.Repository
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
	_ = pathutil.EnsureDirExist(diagnosticsDir)

	diagnosticsFile := filepath.Join(diagnosticsDir, "simulatorDiagnostics.txt")
	_ = fileutil.NewFileManager().Write(diagnosticsFile, "test-diagnostics", 0777)

	exporter, _ := createSutAndMocks()

	// When
	err := exporter.ExportSimulatorDiagnostics(tempDir, diagnosticsDir, name)

	// Then
	assert.NoError(t, err)
	assert.True(t, isPathExists(filepath.Join(tempDir, name+".zip")))
}

// Helpers

func createSutAndMocks() (Exporter, testingMocks) {
	envRepository := new(mockenv.Repository)
	envRepository.On("Set", mock.Anything, mock.Anything).Return(nil)

	exporter := NewExporter(envRepository, log.NewLogger(), nil)

	return exporter, testingMocks{
		envRepository: envRepository,
	}
}

func isPathExists(path string) bool {
	isExist, _ := pathutil.NewPathChecker().IsPathExists(path)
	return isExist
}
