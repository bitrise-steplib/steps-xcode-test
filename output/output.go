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
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/converters/xcresult3"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/converters/xcresult3/model3"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
)

const (
	flakyTestCasesEnvVarKey              = "BITRISE_FLAKY_TEST_CASES"
	flakyTestCasesEnvVarSizeLimitInBytes = 1024
)

// Exporter ...
type Exporter interface {
	ExportXCResultBundle(deployDir, xcResultPath, scheme string)
	ExportTestRunResult(failed bool)
	ExportXcodebuildBuildLog(deployDir, xcodebuildBuildLog string) error
	ExportXcodebuildTestLog(deployDir, xcodebuildTestLog string) error
	ExportSimulatorDiagnostics(deployDir, pth, name string) error
	ExportFlakyTestCases(xcResultPath string, useOldXCResultExtractionMethod bool) error
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

func (e exporter) ExportFlakyTestCases(xcResultPath string, useOldXCResultExtractionMethod bool) error {
	testSummary, err := e.parseTestSummary(xcResultPath, useOldXCResultExtractionMethod)
	if err != nil {
		return fmt.Errorf("failed to parse test summary: %w", err)
	}

	flakyTestPlans := e.collectFlakyTestPlans(*testSummary)
	if len(flakyTestPlans) == 0 {
		return nil
	}

	return e.exportFlakyTestCases(flakyTestPlans)
}

func (e exporter) parseTestSummary(xcResultPath string, useOldXCResultExtractionMethod bool) (*model3.TestSummary, error) {
	converter := xcresult3.Converter{}
	converter.Setup(useOldXCResultExtractionMethod)
	if !converter.Detect([]string{xcResultPath}) {
		return nil, nil
	}

	results, err := xcresult3.ParseTestResults(xcResultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xcresult: %w", err)
	}

	testSummary, warnings, err := model3.Convert(results)
	if err != nil {
		return nil, fmt.Errorf("failed to convert xcresult data: %w", err)
	}

	if len(warnings) > 0 {
		e.logger.Warnf("xcresult converter warnings:")
		for _, warning := range warnings {
			e.logger.Warnf("- %s", warning)
		}
	}

	return testSummary, nil
}

func (e exporter) collectFlakyTestPlans(testSummary model3.TestSummary) []model3.TestPlan {
	var flakyTestPlans []model3.TestPlan
	for _, testPlan := range testSummary.TestPlans {
		var flakyTestBundles []model3.TestBundle

		for _, testBundle := range testPlan.TestBundles {
			var flakyTestSuites []model3.TestSuite

			for _, testSuite := range testBundle.TestSuites {
				var flakyTestCases []model3.TestCaseWithRetries

				for _, testCase := range testSuite.TestCases {
					if testCase.Result != model3.TestResultPassed {
						continue
					}

					if len(testCase.Retries) == 0 {
						continue
					}

					for _, retry := range testCase.Retries {
						if retry.Result == model3.TestResultFailed {
							flakyTestCases = append(flakyTestCases, testCase)
						}
					}
				}

				if len(flakyTestCases) > 0 {
					flakyTestSuites = append(flakyTestSuites, model3.TestSuite{
						Name:      testSuite.Name,
						TestCases: flakyTestCases,
					})
				}
			}

			if len(flakyTestSuites) > 0 {
				flakyTestBundles = append(flakyTestBundles, model3.TestBundle{
					Name:       testBundle.Name,
					TestSuites: flakyTestSuites,
				})
			}
		}

		if len(flakyTestBundles) > 0 {
			flakyTestPlans = append(flakyTestPlans, model3.TestPlan{
				Name:        testPlan.Name,
				TestBundles: flakyTestBundles,
			})
		}
	}

	return flakyTestPlans
}

func (e exporter) exportFlakyTestCases(flakyTestPlans []model3.TestPlan) error {
	if len(flakyTestPlans) == 0 {
		return nil
	}

	storedFlakyTestCases := map[string]bool{}
	var flakyTestCases []string

	for _, testPlan := range flakyTestPlans {
		for _, testBundle := range testPlan.TestBundles {
			for _, testSuite := range testBundle.TestSuites {
				for _, testCase := range testSuite.TestCases {
					testCaseName := testCase.Name
					if len(testCase.ClassName) > 0 {
						testCaseName = fmt.Sprintf("%s.%s", testCase.ClassName, testCase.Name)
					}

					testCaseName = testBundle.Name + "." + testCaseName

					if _, stored := storedFlakyTestCases[testCaseName]; !stored {
						storedFlakyTestCases[testCaseName] = true
						flakyTestCases = append(flakyTestCases, testCaseName)
					}
				}
			}
		}
	}

	var flakyTestCasesMessage string
	for i, flakyTestCase := range flakyTestCases {
		flakyTestCasesMessageLine := fmt.Sprintf("- %s\n", flakyTestCase)

		if len(flakyTestCasesMessage)+len(flakyTestCasesMessageLine) > 1024 {
			e.logger.Warnf("%s env var size limit (1024 characters) exceeded. Skipping %d test cases.", flakyTestCasesEnvVarKey, len(flakyTestCases)-i)
			break
		}

		flakyTestCasesMessage += flakyTestCasesMessageLine
	}

	if err := e.envRepository.Set(flakyTestCasesEnvVarKey, flakyTestCasesMessage); err != nil {
		return fmt.Errorf("failed to export %s: %w", flakyTestCasesEnvVarKey, err)
	}

	return nil
}
