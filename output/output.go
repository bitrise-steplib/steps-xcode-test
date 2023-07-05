package output

import (
	"fmt"
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
