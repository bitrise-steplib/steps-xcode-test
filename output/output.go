package output

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/testresult/xcresult3"
	"github.com/bitrise-io/go-xcode/v2/testresult/xcresult3/model3"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
)

const (
	flakyTestCasesEnvVarKey              = "BITRISE_FLAKY_TEST_CASES"
	flakyTestCasesEnvVarSizeLimitInBytes = 1024
)

// ErrNoDiagnostics is returned by ExportXcresultDiagnostics when the xcresult holds no diagnostics at all,
// which is the case when xcodebuild did not reach the test phase (for example the build failed).
var ErrNoDiagnostics = errors.New("the xcresult holds no diagnostics")

// Exporter ...
type Exporter interface {
	ExportXCResultBundle(deployDir, xcResultPath, scheme string)
	ExportTestRunResult(failed bool)
	ExportXcodebuildBuildLog(deployDir, xcodebuildBuildLog string) error
	ExportXcodebuildTestLog(deployDir, xcodebuildTestLog string) error
	ExportXcresultDiagnostics(deployDir, xcResultPath, scheme string) error
	ExportFlakyTestCases(xcResultPath string, useOldXCResultExtractionMethod bool) error
}

// DirZipper zips a directory into a destination archive.
type DirZipper interface {
	ZipDir(sourceDirPth, destinationZipPth string, isContentOnly bool) error
}

type exporter struct {
	envRepository     env.Repository
	logger            log.Logger
	outputExporter    export.Exporter
	testAddonExporter testaddon.Exporter
	commandFactory    command.Factory
	dirZipper         DirZipper
	pathChecker       pathutil.PathChecker
	pathModifier      pathutil.PathModifier
	pathProvider      pathutil.PathProvider
	fileManager       fileutil.FileManager
}

// NewExporter ...
func NewExporter(envRepository env.Repository, logger log.Logger, outputExporter export.Exporter, testAddonExporter testaddon.Exporter, commandFactory command.Factory, dirZipper DirZipper, pathChecker pathutil.PathChecker, pathModifier pathutil.PathModifier, pathProvider pathutil.PathProvider, fileManager fileutil.FileManager) Exporter {
	return &exporter{
		envRepository:     envRepository,
		logger:            logger,
		outputExporter:    outputExporter,
		testAddonExporter: testAddonExporter,
		commandFactory:    commandFactory,
		dirZipper:         dirZipper,
		pathChecker:       pathChecker,
		pathModifier:      pathModifier,
		pathProvider:      pathProvider,
		fileManager:       fileManager,
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
	if err := e.fileManager.CopyFile(pth, deployPth, &fileutil.CopyOptions{Overwrite: true}); err != nil {
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
	if err := e.fileManager.CopyFile(pth, deployPth, &fileutil.CopyOptions{Overwrite: true}); err != nil {
		return fmt.Errorf("failed to copy xcodebuild output log file from (%s) to (%s): %w", pth, deployPth, err)
	}

	if err := e.envRepository.Set("BITRISE_XCODEBUILD_TEST_LOG_PATH", deployPth); err != nil {
		e.logger.Warnf("Failed to export: BITRISE_XCODEBUILD_TEST_LOG_PATH: %s", err)
	}

	return nil
}

// ExportXcresultDiagnostics exports the Simulator diagnostics xcodebuild collected into the xcresult
// (-collect-test-diagnostics) and zips them into the deploy dir as Test-<scheme>-diagnostics.zip.
func (e exporter) ExportXcresultDiagnostics(deployDir, xcResultPath, scheme string) error {
	exists, err := e.pathChecker.IsPathExists(xcResultPath)
	if err != nil {
		return fmt.Errorf("failed to check the xcresult: %w", err)
	}
	if !exists { // xcodebuild did not get as far as writing the result bundle
		return fmt.Errorf("%w: no xcresult at %s", ErrNoDiagnostics, xcResultPath)
	}

	outputDir, err := e.pathProvider.CreateTempDir("xcresult-diagnostics")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory: %w", err)
	}
	defer func() {
		if err := e.fileManager.RemoveAll(outputDir); err != nil {
			e.logger.Warnf("Failed to remove the temporary diagnostics directory (%s): %s", outputDir, err)
		}
	}()

	cmd := e.commandFactory.Create("xcrun", []string{"xcresulttool", "export", "diagnostics", "--path", xcResultPath, "--output-path", outputDir}, nil)
	e.logger.Printf("$ %s", cmd.PrintableCommandArgs())

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("xcresulttool export diagnostics failed: %w, output: %s", err, out)
	}
	e.logger.Debugf("%s", out)

	entries, err := e.fileManager.ReadDirEntryNames(outputDir)
	if err != nil {
		return fmt.Errorf("failed to read the exported diagnostics: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("%w: %s", ErrNoDiagnostics, xcResultPath)
	}

	// xcresulttool exports the test session logs even when xcodebuild did not collect the Simulator diagnostics
	// (for example when the test plan says never); export what is there and say what is missing.
	hasBundle, err := e.hasSimulatorDiagnostics(outputDir)
	if err != nil {
		return fmt.Errorf("failed to inspect the exported diagnostics: %w", err)
	}
	if !hasBundle {
		e.logger.Warnf("The xcresult contains no Simulator diagnostics, exporting the test session logs only")
	}

	zipPath := filepath.Join(deployDir, fmt.Sprintf("Test-%s-diagnostics.zip", scheme))
	if err := e.dirZipper.ZipDir(outputDir, zipPath, true); err != nil {
		// do not leave a partial archive behind for the deploy step
		if removeErr := e.fileManager.Remove(zipPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			e.logger.Warnf("Failed to remove the partial archive (%s): %s", zipPath, removeErr)
		}
		return fmt.Errorf("failed to compress the exported diagnostics: %w", err)
	}

	e.logger.Donef("Simulator diagnostics are available as an artifact (%s)", zipPath)

	return nil
}

// hasSimulatorDiagnostics tells whether the exported diagnostics hold a simctl diagnose bundle. xcresulttool writes
// one directory per test action (for example "0_Test_iPhone 17_Diagnostics") and the bundle as simctl_diagnostics
// inside it (checked on Xcode 26.5 and 27.0).
func (e exporter) hasSimulatorDiagnostics(outputDir string) (bool, error) {
	matches, err := e.pathProvider.Glob(filepath.Join(e.pathModifier.EscapeGlobPath(outputDir), "*", "simctl_diagnostics"))
	if err != nil {
		return false, err
	}
	return len(matches) > 0, nil
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
	converter := xcresult3.NewConverter(useOldXCResultExtractionMethod)
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

		if len(flakyTestCasesMessage)+len(flakyTestCasesMessageLine) > flakyTestCasesEnvVarSizeLimitInBytes {
			e.logger.Warnf("%s env var size limit (%d characters) exceeded. Skipping %d test cases.", flakyTestCasesEnvVarKey, flakyTestCasesEnvVarSizeLimitInBytes, len(flakyTestCases)-i)
			break
		}

		flakyTestCasesMessage += flakyTestCasesMessageLine
	}

	if err := e.envRepository.Set(flakyTestCasesEnvVarKey, flakyTestCasesMessage); err != nil {
		return fmt.Errorf("failed to export %s: %w", flakyTestCasesEnvVarKey, err)
	}

	return nil
}
