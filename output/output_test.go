package output

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
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

// Helpers

func createSutAndMocks() (Exporter, testingMocks) {
	envRepository := new(mocks.Repository)
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
