package output

import (
	"os"
	"path/filepath"

	"github.com/bitrise-steplib/steps-xcode-test/testartifact"

	"github.com/bitrise-io/go-utils/ziputil"

	"github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/go-steputils/output"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
)

type Exporter interface {
	ExportXCResultBundle(deployDir, xcResultPath, scheme string)
	ExportTestRunResult(failed bool)
	ExportXcodebuildBuildLog(deployDir, xcodebuildBuildLog string)
	ExportXcodebuildTestLog(deployDir, xcodebuildTestLog string)
	ExportSimulatorDiagnostics(deployDir, pth, name string)
	ExportUITestArtifacts(xcResultPath, scheme string)
}

type exporter struct {
	envRepository        env.Repository
	logger               log.Logger
	testAddonExporter    testaddon.Exporter
	testArtifactExporter testartifact.Exporter
}

func NewExporter(envRepository env.Repository, logger log.Logger, testAddonExporter testaddon.Exporter, testArtifactExporter testartifact.Exporter) Exporter {
	return &exporter{
		envRepository:        envRepository,
		logger:               logger,
		testAddonExporter:    testAddonExporter,
		testArtifactExporter: testArtifactExporter,
	}
}

func (e exporter) ExportTestRunResult(failed bool) {
	status := "succeeded"
	if failed {
		status = "failed"
	}
	if err := e.envRepository.Set("BITRISE_XCODE_TEST_RESULT", status); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}
}

func (e exporter) ExportXCResultBundle(deployDir, xcResultPath, scheme string) {
	// export xcresult bundle
	if err := e.envRepository.Set("BITRISE_XCRESULT_PATH", xcResultPath); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCRESULT_PATH, error: %s", err)
	}

	xcresultZipPath := filepath.Join(deployDir, filepath.Base(xcResultPath)+".zip")
	if err := output.ZipAndExportOutput(xcResultPath, xcresultZipPath, "BITRISE_XCRESULT_ZIP_PATH"); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCRESULT_ZIP_PATH, error: %s", err)
	}

	// export xcresult for the testing addon
	if addonResultPath := os.Getenv(configs.BitrisePerStepTestResultDirEnvKey); len(addonResultPath) > 0 {
		e.logger.Println()
		e.logger.Infof("Exporting test results")

		if err := e.testAddonExporter.CopyAndSaveMetadata(testaddon.AddonCopy{
			SourceTestOutputDir:   xcResultPath,
			TargetAddonPath:       addonResultPath,
			TargetAddonBundleName: scheme,
		}); err != nil {
			e.logger.Warnf("Failed to export test results, error: %s", err)
		}
	}
}

func (e exporter) ExportXcodebuildBuildLog(deployDir, xcodebuildBuildLog string) {
	pth, err := saveRawOutputToLogFile(xcodebuildBuildLog)
	if err != nil {
		e.logger.Warnf("Failed to save the Raw Output, err: %s", err)
	}

	deployPth := filepath.Join(deployDir, "xcodebuild_build.log")
	if err := command.CopyFile(pth, deployPth); err != nil {
		// TODO: restore error handling
		e.logger.Warnf("failed to copy xcodebuild output log file from (%s) to (%s), error: %s", pth, deployPth, err)
		return
	}

	if err := e.envRepository.Set("BITRISE_XCODEBUILD_BUILD_LOG_PATH", deployPth); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODEBUILD_BUILD_LOG_PATH, error: %s", err)
	}
}

func (e exporter) ExportXcodebuildTestLog(deployDir, xcodebuildTestLog string) {
	pth, err := saveRawOutputToLogFile(xcodebuildTestLog)
	if err != nil {
		e.logger.Warnf("Failed to save the Raw Output, error: %s", err)
	}

	deployPth := filepath.Join(deployDir, "xcodebuild_test.log")
	if err := command.CopyFile(pth, deployPth); err != nil {
		// TODO: restore error handling
		e.logger.Warnf("failed to copy xcodebuild output log file from (%s) to (%s), error: %s", pth, deployPth, err)
		return
	}

	if err := e.envRepository.Set("BITRISE_XCODEBUILD_TEST_LOG_PATH", deployPth); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODEBUILD_TEST_LOG_PATH, error: %s", err)
	}
}

func (e exporter) ExportSimulatorDiagnostics(deployDir, pth, name string) {
	outputPath := filepath.Join(deployDir, name)
	if err := ziputil.ZipDir(pth, outputPath, true); err != nil {
		// TODO: restore error handling
		e.logger.Warnf("failed to compress simulator diagnostics result: %v", err)
		return
	}
}

func (e exporter) ExportUITestArtifacts(xcResultPath, scheme string) {
	// The test result bundle (xcresult) structure changed in Xcode 11:
	// it does not contains TestSummaries.plist nor Attachments directly.
	e.logger.Println()
	e.logger.Infof("Exporting attachments")

	testSummariesPath, attachementDir, err := e.testArtifactExporter.GetSummariesAndAttachmentPath(xcResultPath)
	if err != nil {
		e.logger.Warnf("Failed to export UI test artifacts, error: %s", err)
		return
	}

	zipedTestsDerivedDataPath, err := e.testArtifactExporter.SaveAttachments(scheme, testSummariesPath, attachementDir)
	if err != nil {
		e.logger.Warnf("Failed to export UI test artifacts, error: %s", err)
		return
	}

	if err := e.envRepository.Set("BITRISE_XCODE_TEST_ATTACHMENTS_PATH", zipedTestsDerivedDataPath); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODE_TEST_ATTACHMENTS_PATH, error: %s", err)
		return
	}

	e.logger.Donef("The zipped attachments are available in: %s", zipedTestsDerivedDataPath)
}
