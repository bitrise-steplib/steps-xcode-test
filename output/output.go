package output

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/ziputil"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
)

// Exporter ...
type Exporter interface {
	ExportXCResultBundle(deployDir, xcResultPath, scheme string)
	ExportTestRunResult(failed bool)
	ExportXcodebuildBuildLog(deployDir, xcodebuildBuildLog string) error
	ExportXcodebuildTestLog(deployDir, xcodebuildTestLog string) error
	ExportSimulatorDiagnostics(deployDir, pth, name string) error
}

type exporter struct {
	envRepository     env.Repository
	logger            log.Logger
	outputExporter    export.Exporter
	testAddonExporter testaddon.Exporter
}

type TestSummary struct {
	Actions []struct {
		ActionResult struct {
			TestsRef struct {
				Id struct {
					Value string `json:"_value"`
				} `json:"id"`
			} `json:"testsRef"`
		} `json:"actionResult"`
	} `json:"actions"`
}

type TestResults struct {
	Summary struct {
		TestsPassedCount struct {
			Value int `json:"_value"`
		} `json:"testsPassedCount"`
		TestsFailedCount struct {
			Value int `json:"_value"`
		} `json:"testsFailedCount"`
	} `json:"summary"`
}

// NewExporter ...
func NewExporter(envRepository env.Repository, logger log.Logger, outputExporter export.Exporter, testAddonExporter testaddon.Exporter) Exporter {
	return &exporter{
		envRepository:     envRepository,
		logger:            logger,
		outputExporter:    outputExporter,
		testAddonExporter: testAddonExporter,
	}
}

func (e exporter) ExportTestRunResult(failed bool) {
	status := "succeeded"
	if failed {
		status = "failed"
	}
	if err := e.envRepository.Set("BITRISE_XCODE_TEST_RESULT", status); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT: %s", err)
	}
}

func (e exporter) ExportXCResultBundle(deployDir, xcResultPath, scheme string) {
	// export xcresult bundle
	if err := e.envRepository.Set("BITRISE_XCRESULT_PATH", xcResultPath); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCRESULT_PATH: %s", err)
	}

	xcresultZipPath := filepath.Join(deployDir, filepath.Base(xcResultPath)+".zip")
	if err := e.outputExporter.ExportOutputFilesZip("BITRISE_XCRESULT_ZIP_PATH", []string{xcResultPath}, xcresultZipPath); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCRESULT_ZIP_PATH: %s", err)
	}

	// export xcresult for the testing addon
	if addonResultPath := e.envRepository.Get(configs.BitrisePerStepTestResultDirEnvKey); len(addonResultPath) > 0 {
		e.logger.Println()
		e.logger.Infof("Exporting test results")

		if err := e.testAddonExporter.CopyAndSaveMetadata(testaddon.AddonCopy{
			SourceTestOutputDir:   xcResultPath,
			TargetAddonPath:       addonResultPath,
			TargetAddonBundleName: scheme,
		}); err != nil {
			e.logger.Warnf("Failed to export test results: %s", err)
		}
	}

	// Parse the test results and set the environment variables
	if err := e.ExportTestResults(xcResultPath); err != nil {
		e.logger.Warnf("Failed to export test results: %s", err)
	}
}

func (e exporter) ExportXcodebuildBuildLog(deployDir, xcodebuildBuildLog string) error {
	pth, err := saveRawOutputToLogFile(xcodebuildBuildLog)
	if err != nil {
		e.logger.Warnf("Failed to save the Raw Output, err: %s", err)
	}

	deployPth := filepath.Join(deployDir, "xcodebuild_build.log")
	if err := command.CopyFile(pth, deployPth); err != nil {
		return fmt.Errorf("failed to copy xcodebuild output log file from (%s) to (%s): %w", pth, deployPth, err)
	}

	if err := e.envRepository.Set("BITRISE_XCODEBUILD_BUILD_LOG_PATH", deployPth); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODEBUILD_BUILD_LOG_PATH: %s", err)
	}

	return nil
}

func (e exporter) ExportXcodebuildTestLog(deployDir, xcodebuildTestLog string) error {
	pth, err := saveRawOutputToLogFile(xcodebuildTestLog)
	if err != nil {
		e.logger.Warnf("Failed to save the Raw Output: %s", err)
	}

	deployPth := filepath.Join(deployDir, "xcodebuild_test.log")
	if err := command.CopyFile(pth, deployPth); err != nil {
		return fmt.Errorf("failed to copy xcodebuild output log file from (%s) to (%s): %w", pth, deployPth, err)
	}

	if err := e.envRepository.Set("BITRISE_XCODEBUILD_TEST_LOG_PATH", deployPth); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODEBUILD_TEST_LOG_PATH: %s", err)
	}

	return nil
}

func (e exporter) ExportSimulatorDiagnostics(deployDir, pth, name string) error {
	outputPath := filepath.Join(deployDir, name)
	if err := ziputil.ZipDir(pth, outputPath, true); err != nil {
		return fmt.Errorf("failed to compress simulator diagnostics result: %w", err)
	}

	return nil
}

func (e exporter) ExportTestResults(xcResultPath string) error {
	// Convert xcresult to JSON
	cmd := exec.Command("xcrun", "xcresulttool", "get", "--path", xcResultPath, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error running xcresulttool: %w", err)
	}

	// Parse JSON output to get the test summary ID
	var summary TestSummary
	err = json.Unmarshal(output, &summary)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %w", err)
	}

	testSummaryId := summary.Actions[0].ActionResult.TestsRef.Id.Value

	// Get test results using the test summary ID
	cmd = exec.Command("xcrun", "xcresulttool", "get", "--path", xcResultPath, "--id", testSummaryId, "--format", "json")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("error running xcresulttool for test results: %w", err)
	}

	// Parse JSON output to get the test results
	var results TestResults
	err = json.Unmarshal(output, &results)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %w", err)
	}

	// Set environment variables for the number of tests passed and failed
	passedCount := results.Summary.TestsPassedCount.Value
	failedCount := results.Summary.TestsFailedCount.Value

	if err := e.envRepository.Set("BITRISE_XCODE_TESTS_PASSED_COUNT", fmt.Sprintf("%d", passedCount)); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODE_TESTS_PASSED_COUNT: %s", err)
	}

	if err := e.envRepository.Set("BITRISE_XCODE_TESTS_FAILED_COUNT", fmt.Sprintf("%d", failedCount)); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODE_TESTS_FAILED_COUNT: %s", err)
	}

	e.logger.Printf("Number of tests passed: %d", passedCount)
	e.logger.Printf("Number of tests failed: %d", failedCount)

	return nil
}
