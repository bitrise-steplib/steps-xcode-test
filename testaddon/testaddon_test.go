package testaddon

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"

	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenNormalBundleName_WhenExport_ThenCreatesOutputStructure(t *testing.T) {
	runTest(t, "Bitrise", "Bitrise")
}

func Test_GivenBundleNameWithSpecialCharacters_WhenExport_ThenReplacesSpecialCharacters(t *testing.T) {
	runTest(t, "W/eir/d:Na::me/", "W-eir-d-Na--me-")
}

func runTest(t *testing.T, bundleName string, expectedBundleName string) {
	// Given
	resultDir, outputDir := prepareArtifacts(t)

	exporter := NewExporter(NewTestAddon(log.NewLogger()))

	// When
	err := exporter.CopyAndSaveMetadata(AddonCopy{
		SourceTestOutputDir:   resultDir,
		TargetAddonPath:       outputDir,
		TargetAddonBundleName: bundleName,
	})

	// Then
	assert.NoError(t, err)
	assert.True(t, isOutputStructureCorrectWithExpectedBundleName(outputDir, expectedBundleName))
}

func prepareArtifacts(t *testing.T) (string, string) {
	tempDir := t.TempDir()

	resultDir := filepath.Join(tempDir, "result")

	xcresultFile := filepath.Join(resultDir, "test.xcresult")
	err := fileutil.NewFileManager().Write(xcresultFile, "test-results", 0777)
	require.NoError(t, err)
	require.FileExists(t, xcresultFile)

	outputDir := filepath.Join(tempDir, "output")

	return resultDir, outputDir
}

func isOutputStructureCorrectWithExpectedBundleName(outputDir string, bundleName string) bool {
	jsonPath := filepath.Join(outputDir, bundleName, "test-info.json")
	expectedPaths := []string{
		filepath.Join(outputDir, bundleName),
		filepath.Join(outputDir, bundleName, "result", "test.xcresult"),
		jsonPath,
	}

	for _, path := range expectedPaths {
		if isPathExists(path) == false {
			return false
		}
	}

	return exportedBundleNameFromFile(jsonPath) == bundleName
}

func exportedBundleNameFromFile(path string) string {
	type testBundle struct {
		BundleName string `json:"test-name"`
	}

	jsonFile, _ := os.Open(path)

	defer jsonFile.Close()

	bytes, _ := io.ReadAll(jsonFile)

	var bundle testBundle
	_ = json.Unmarshal(bytes, &bundle)

	return bundle.BundleName
}

func isPathExists(path string) bool {
	isExist, _ := pathutil.NewPathChecker().IsPathExists(path)
	return isExist
}
