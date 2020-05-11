package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	bitriseConfigs "github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/stringutil"
	simulator "github.com/bitrise-io/go-xcode/simulator"
	"github.com/bitrise-io/go-xcode/utility"
	cache "github.com/bitrise-io/go-xcode/xcodecache"
	cmd "github.com/bitrise-steplib/steps-xcode-test/command"
	"github.com/bitrise-steplib/steps-xcode-test/models"
	shellquote "github.com/kballard/go-shellquote"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.

const (
	minSupportedXcodeMajorVersion = 6
	// This boot timeout can happen when running Unit Tests with Xcode Command Line `xcodebuild`.
	timeOutMessageIPhoneSimulator = "iPhoneSimulator: Timed out waiting"
	// This boot timeout can happen when running Xcode (7+) UI tests with Xcode Command Line `xcodebuild`.
	timeOutMessageUITest                     = "Terminating app due to uncaught exception '_XCTestCaseInterruptionException'"
	earlyUnexpectedExit                      = "Early unexpected exit, operation never finished bootstrapping - no restart will be attempted"
	failureAttemptingToLaunch                = "Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl:"
	failedToBackgroundTestRunner             = `Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner.`
	appStateIsStillNotRunning                = `App state is still not running active, state = XCApplicationStateNotRunning`
	appAccessibilityIsNotLoaded              = `UI Testing Failure - App accessibility isn't loaded`
	testRunnerFailedToInitializeForUITesting = `Test runner failed to initialize for UI testing`
	timedOutRegisteringForTestingEvent       = `Timed out registering for testing event accessibility notifications`
)

var automaticRetryReasonPatterns = []string{
	timeOutMessageIPhoneSimulator,
	timeOutMessageUITest,
	earlyUnexpectedExit,
	failureAttemptingToLaunch,
	failedToBackgroundTestRunner,
	appStateIsStillNotRunning,
	appAccessibilityIsNotLoaded,
	testRunnerFailedToInitializeForUITesting,
	timedOutRegisteringForTestingEvent,
}

var xcodeCommandEnvs = []string{"NSUnbufferedIO=YES"}

// -----------------------
// --- Models
// -----------------------

// Configs ...
type Configs struct {
	// Project Parameters
	ProjectPath string `env:"project_path,required"`
	Scheme      string `env:"scheme,required"`

	// Simulator Configs
	SimulatorPlatform  string `env:"simulator_platform,required"`
	SimulatorDevice    string `env:"simulator_device,required"`
	SimulatorOsVersion string `env:"simulator_os_version,required"`

	// Test Run Configs
	OutputTool    string `env:"output_tool,opt[xcpretty,xcodebuild]"`
	IsCleanBuild  bool   `env:"is_clean_build,opt[yes,no]"`
	IsSingleBuild bool   `env:"single_build,opt[true,false]"`

	ShouldBuildBeforeTest         bool `env:"should_build_before_test,opt[yes,no]"`
	ShouldRetryTestOnFail         bool `env:"should_retry_test_on_fail,opt[yes,no]"`
	ShouldRetryIndividualFailures bool `env:"should_retry_individual_failures,opt[yes,no]"`
	TestFailureRetryLimit         int  `env:"test_failure_retry_limit,opt[1,2,3,4,5]"`

	GenerateCodeCoverageFiles bool `env:"generate_code_coverage_files,opt[yes,no]"`
	ExportUITestArtifacts     bool `env:"export_uitest_artifacts,opt[true,false]"`

	DisableIndexWhileBuilding bool `env:"disable_index_while_building,opt[yes,no]"`

	// Not required parameters
	TestOptions         string `env:"xcodebuild_test_options"`
	XcprettyTestOptions string `env:"xcpretty_test_options"`

	// Debug
	Verbose      bool `env:"verbose,opt[yes,no]"`
	HeadlessMode bool `env:"headless_mode,opt[yes,no]"`

	CacheLevel string `env:"cache_level,opt[none,swift_packages]"`
}

func isStringFoundInOutput(searchStr, outputToSearchIn string) bool {
	r, err := regexp.Compile("(?i)" + searchStr)
	if err != nil {
		log.Warnf("Failed to compile regexp: %s", err)
		return false
	}
	return r.MatchString(outputToSearchIn)
}

func runXcodeBuildCmd(useStdOut bool, args ...string) (string, int, error) {
	// command
	buildCmd := cmd.CreateXcodebuildCmd(args...)
	// output buffer
	var outBuffer bytes.Buffer
	// additional output writers, like StdOut
	outWritters := []io.Writer{}
	if useStdOut {
		outWritters = append(outWritters, os.Stdout)
	}
	// unify as a single writer
	outWritter := cmd.CreateBufferedWriter(&outBuffer, outWritters...)
	// and set the writer
	buildCmd.Stdin = nil
	buildCmd.Stdout = outWritter
	buildCmd.Stderr = outWritter
	buildCmd.Env = append(os.Environ(), xcodeCommandEnvs...)

	cmdArgsForPrint := cmd.PrintableCommandArgsWithEnvs(buildCmd.Args, xcodeCommandEnvs)

	log.Printf("$ %s", cmdArgsForPrint)

	err := buildCmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return outBuffer.String(), 1, errors.New("failed to cast exit status")
			}
			return outBuffer.String(), waitStatus.ExitStatus(), err
		}
		return outBuffer.String(), 1, err
	}
	return outBuffer.String(), 0, nil
}

func runPrettyXcodeBuildCmd(useStdOut bool, xcprettyArgs []string, xcodebuildArgs []string) (string, int, error) {
	//
	buildCmd := cmd.CreateXcodebuildCmd(xcodebuildArgs...)
	prettyCmd := cmd.CreateXcprettyCmd(xcprettyArgs...)
	//
	var buildOutBuffer bytes.Buffer
	//
	pipeReader, pipeWriter := io.Pipe()
	//
	// build outputs:
	// - write it into a buffer
	// - write it into the pipe, which will be fed into xcpretty
	buildOutWriters := []io.Writer{pipeWriter}
	buildOutWriter := cmd.CreateBufferedWriter(&buildOutBuffer, buildOutWriters...)
	//
	var prettyOutWriter io.Writer
	if useStdOut {
		prettyOutWriter = os.Stdout
	}

	// and set the writers
	buildCmd.Stdin = nil
	buildCmd.Stdout = buildOutWriter
	buildCmd.Stderr = buildOutWriter
	//
	prettyCmd.Stdin = pipeReader
	prettyCmd.Stdout = prettyOutWriter
	prettyCmd.Stderr = prettyOutWriter
	//
	buildCmd.Env = append(os.Environ(), xcodeCommandEnvs...)

	log.Printf("$ set -o pipefail && %s | %v",
		cmd.PrintableCommandArgsWithEnvs(buildCmd.Args, xcodeCommandEnvs),
		cmd.PrintableCommandArgs(prettyCmd.Args))

	fmt.Println()

	if err := buildCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}
	if err := prettyCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}

	defer func() {
		if err := pipeWriter.Close(); err != nil {
			log.Warnf("Failed to close xcodebuild-xcpretty pipe, error: %s", err)
		}

		if err := prettyCmd.Wait(); err != nil {
			log.Warnf("xcpretty command failed, error: %s", err)
		}
	}()

	if err := buildCmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return buildOutBuffer.String(), 1, errors.New("failed to cast exit status")
			}
			return buildOutBuffer.String(), waitStatus.ExitStatus(), err
		}
		return buildOutBuffer.String(), 1, err
	}

	return buildOutBuffer.String(), 0, nil
}

func runBuild(buildParams models.XcodeBuildParamsModel, outputTool string) (string, int, error) {
	xcodebuildArgs := []string{buildParams.Action, buildParams.ProjectPath, "-scheme", buildParams.Scheme}
	if buildParams.CleanBuild {
		xcodebuildArgs = append(xcodebuildArgs, "clean")
	}

	// Disable indexing during the build.
	// Indexing is needed for autocomplete, ability to quickly jump to definition, get class and method help by alt clicking.
	// Which are not needed in CI environment.
	if buildParams.DisableIndexWhileBuilding {
		xcodebuildArgs = append(xcodebuildArgs, "COMPILER_INDEX_STORE_ENABLE=NO")
	}
	xcodebuildArgs = append(xcodebuildArgs, "build", "-destination", buildParams.DeviceDestination)

	log.Infof("Building the project...")

	if outputTool == "xcpretty" {
		return runPrettyXcodeBuildCmd(false, []string{}, xcodebuildArgs)
	}
	return runXcodeBuildCmd(false, xcodebuildArgs...)
}

func runTest(buildTestParams models.XcodeBuildTestParamsModel, outputTool, xcprettyOptions string, isAutomaticRetryOnReason, isRetryOnFail, isRetryIndividualFailures bool, retryLimit, retryAttempt int, swiftPackagesPath string) (string, int, error) {
	handleTestError := func(fullOutputStr string, exitCode int, testError error) (string, int, error) {
		if swiftPackagesPath != "" && isStringFoundInOutput(cache.SwiftPackagesStateInvalid, fullOutputStr) {
			log.RWarnf("xcode-test", "swift-packages-cache-invalid", nil, "swift packages cache is in an invalid state")
			if err := os.RemoveAll(swiftPackagesPath); err != nil {
				log.Errorf("failed to remove Swift package caches, error: %s", err)
				return fullOutputStr, exitCode, testError
			}
		}

		currentAttempt := retryAttempt + 1

		// If the currentAttempt is greater than the limit, we are out of retries so we will skip out.
		// Or if there are no retries set we will also skip out.
		if (currentAttempt > retryLimit) || !(isAutomaticRetryOnReason || isRetryIndividualFailures || isRetryOnFail) {
			log.Errorf("isAutomaticRetryOnReason=%v, isRetryOnFail=%v, isRetryIndividualFailures=%v no more retry, stopping the test!", isAutomaticRetryOnReason, isRetryOnFail, isRetryIndividualFailures)
			return fullOutputStr, exitCode, testError
		}

		//
		// Automatic retry
		for _, retryReasonPattern := range automaticRetryReasonPatterns {
			if isStringFoundInOutput(retryReasonPattern, fullOutputStr) {
				log.Warnf("Automatic retry reason found in log: %s", retryReasonPattern)
				if isAutomaticRetryOnReason {
					log.Printf("isAutomaticRetryOnReason=true - retrying... attempt: %d of %d", currentAttempt, retryLimit)
					return runTest(buildTestParams, outputTool, xcprettyOptions, isAutomaticRetryOnReason, isRetryOnFail, isRetryIndividualFailures, retryLimit, currentAttempt, swiftPackagesPath)
				}
			}
		}

		// Retry individual failures superceeds entire trest retry. This is designed to keep from unnecessarily building the entire
		// target a second time and running all of the other tests, that have already passed, again.
		if xcodeBuildVersion, err := utility.GetXcodeVersion(); err == nil && xcodeBuildVersion.MajorVersion >= 11 {
			if isRetryIndividualFailures {
				log.Warnf("Test run fialed")

				failurePaths := getFailureResults(buildTestParams.TestOutputDir)
				onlyTestOpts := getAdditionalOptionsFromTestFailures(failurePaths)

				// We need to ensure that there are actually test failures here. If there are, we know that the build
				// was successful and we can proceed with the retry without building again. If there are no test failures,
				// we assume that there was an issue with the build, so we want to allow rebuilding.
				if len(failurePaths) > 0 {
					newOutputDir := ""
					lastSlashIndex := strings.LastIndex(buildTestParams.TestOutputDir, "/")

					if lastSlashIndex >= 0 {
						newOutputDir = buildTestParams.TestOutputDir[0:lastSlashIndex]
					}

					newOutputDir = path.Join(newOutputDir, "Test_Retry_" + string(currentAttempt) + ".xcresult")

					newBuildTestParams := models.XcodeBuildTestParamsModel{
						buildTestParams.BuildParams,
						newOutputDir,
						false, // Clean build is false
						false, // Build before test is false
						buildTestParams.GenerateCodeCoverage,
						buildTestParams.AdditionalOptions,
						onlyTestOpts,
					}

					log.Printf("isRetryIndividualFailures=true - retrying... attempt: %d of %d", currentAttempt, retryLimit)
					return runTest(newBuildTestParams, outputTool, xcprettyOptions, isAutomaticRetryOnReason, isRetryOnFail, isRetryIndividualFailures, retryLimit, currentAttempt, swiftPackagesPath)
				}
			}
		}

		//
		// If we are here it must be Retry on fail
		log.Warnf("Test run failed")
		log.Printf("isRetryOnFail=true - retrying...")
		return runTest(buildTestParams, outputTool, xcprettyOptions, isAutomaticRetryOnReason, isRetryOnFail, isRetryIndividualFailures, retryLimit, currentAttempt, swiftPackagesPath)
	}

	// Clean output directory, otherwise after retry test run, xcodebuild fails with `error: Existing file at -resultBundlePath "..."`
	if err := os.RemoveAll(buildTestParams.TestOutputDir); err != nil {
		return "", 1, fmt.Errorf("failed to clean test output directory: %s, error: %s", buildTestParams.TestOutputDir, err)
	}
	buildParams := buildTestParams.BuildParams

	xcodebuildArgs := []string{buildParams.Action, buildParams.ProjectPath, "-scheme", buildParams.Scheme}
	if buildTestParams.CleanBuild {
		xcodebuildArgs = append(xcodebuildArgs, "clean")
	}
	// the 'build' argument is required *before* the 'test' arg, to prevent
	//  the Xcode bug described in the README, which causes:
	// 'iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.'
	//  in case the compilation takes a long time.
	// Related Radar link: https://openradar.appspot.com/22413115
	// Demonstration project: https://github.com/bitrise-io/simulator-launch-timeout-includes-build-time

	// for builds < 120 seconds or fixed Xcode versions, one should
	// have the possibility of opting out, because the explicit build arg
	// leads the project to be compiled twice and increase the duration
	// Related issue link: https://github.com/bitrise-steplib/steps-xcode-test/issues/55
	if buildTestParams.BuildBeforeTest {
		xcodebuildArgs = append(xcodebuildArgs, "build")
		xcodebuildArgs = append(xcodebuildArgs, "test", "-destination", buildParams.DeviceDestination)
	} else {
		xcodebuildArgs = append(xcodebuildArgs, "test-without-building", "-destination", buildParams.DeviceDestination)
	}

	// Disable indexing during the build.
	// Indexing is needed for autocomplete, ability to quickly jump to definition, get class and method help by alt clicking.
	// Which are not needed in CI environment.
	if buildParams.DisableIndexWhileBuilding {
		xcodebuildArgs = append(xcodebuildArgs, "COMPILER_INDEX_STORE_ENABLE=NO")
	}

	xcodebuildArgs = append(xcodebuildArgs, "-resultBundlePath", buildTestParams.TestOutputDir)

	if buildTestParams.GenerateCodeCoverage {
		xcodebuildArgs = append(xcodebuildArgs, "GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES")
		xcodebuildArgs = append(xcodebuildArgs, "GCC_GENERATE_TEST_COVERAGE_FILES=YES")
	}

	if buildTestParams.AdditionalOptions != "" {
		options, err := parseAdditionalOptions(buildTestParams.AdditionalOptions)

		if err != nil {
			return "", 1, err
		}

		xcodebuildArgs = append(xcodebuildArgs, options...)
	}

	if buildTestParams.OnlyTestOptions != "" {
		options, err := parseAdditionalOptions(buildTestParams.OnlyTestOptions)

		if err != nil {
			return "", 1, err
		}

		xcodebuildArgs = append(xcodebuildArgs, options...)
	}

	xcprettyArgs := []string{}
	if xcprettyOptions != "" {
		options, err := shellquote.Split(xcprettyOptions)
		if err != nil {
			return "", 1, fmt.Errorf("failed to parse additional options (%s), error: %s", xcprettyOptions, err)
		}
		// get and delete the xcpretty output file, if exists
		xcprettyOutputFilePath := ""
		isNextOptOutputPth := false
		for _, aOpt := range options {
			if isNextOptOutputPth {
				xcprettyOutputFilePath = aOpt
				break
			}
			if aOpt == "--output" {
				isNextOptOutputPth = true
				continue
			}
		}
		if xcprettyOutputFilePath != "" {
			if isExist, err := pathutil.IsPathExists(xcprettyOutputFilePath); err != nil {
				log.Errorf("Failed to check xcpretty output file status (path: %s), error: %s", xcprettyOutputFilePath, err)
			} else if isExist {
				log.Warnf("=> Deleting existing xcpretty output: %s", xcprettyOutputFilePath)
				if err := os.Remove(xcprettyOutputFilePath); err != nil {
					log.Errorf("Failed to delete xcpretty output file (path: %s), error: %s", xcprettyOutputFilePath, err)
				}
			}
		}
		//
		xcprettyArgs = append(xcprettyArgs, options...)
	}

	log.Infof("Running the tests...")

	var rawOutput string
	var err error
	var exit int
	if outputTool == "xcpretty" {
		rawOutput, exit, err = runPrettyXcodeBuildCmd(true, xcprettyArgs, xcodebuildArgs)
	} else {
		rawOutput, exit, err = runXcodeBuildCmd(true, xcodebuildArgs...)
	}

	if err != nil {
		return handleTestError(rawOutput, exit, err)
	}
	return rawOutput, exit, nil
}

// Returns a slice of the individual test cases that failed.
func getFailureResults(testResultPath string) []string {
	var out bytes.Buffer

	cmd := cmd.CreateXcTestResultCmd(testResultPath)
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		log.Errorf("There was an error getting the failure results: %v", err)
	}

	var testResult models.XcodeActionsInvocationRecord
	if err := json.Unmarshal(out.Bytes(), &testResult); err != nil {
		log.Errorf("There was an error Unmarshaling the test result: %v", err)
	}

	return testResult.TestFailures()
}

// Returns a string of "only-testing" options from the individal test failures
// Note: This method will escape spaces in test target names.
func getAdditionalOptionsFromTestFailures(failures []string) string {
	opts := ""

	for _, path := range failures {
		escaped := strings.ReplaceAll(path, " ", "\\ ")
		opts += "-only-testing:" + escaped + " "
	}

	return strings.Trim(opts, " ")
}

func parseAdditionalOptions(options string) ([]string, error) {
	parsedOptions, err := shellquote.Split(options)

	if err != nil {
		return nil, fmt.Errorf("failed to parse additional options (%s), error: %s", options, err)
	}

	return parsedOptions, err
}

func saveRawOutputToLogFile(rawXcodebuildOutput string, isRunSuccess bool) (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("xcodebuild-output")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir, error: %s", err)
	}
	logFileName := "raw-xcodebuild-output.log"
	logPth := filepath.Join(tmpDir, logFileName)
	if err := fileutil.WriteStringToFile(logPth, rawXcodebuildOutput); err != nil {
		return "", fmt.Errorf("failed to write xcodebuild output to file, error: %s", err)
	}

	if !isRunSuccess {
		deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
		if deployDir == "" {
			return "", errors.New("no BITRISE_DEPLOY_DIR found")
		}
		deployPth := filepath.Join(deployDir, logFileName)

		if err := command.CopyFile(logPth, deployPth); err != nil {
			return "", fmt.Errorf("failed to copy xcodebuild output log file from (%s) to (%s), error: %s", logPth, deployPth, err)
		}
		logPth = deployPth
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH", logPth); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH, error: %s", err)
	}
	return logPth, nil
}

func saveAttachments(scheme, testSummariesPath, attachementDir string) error {
	if exist, err := pathutil.IsDirExists(attachementDir); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("no test attachments found at: %s", attachementDir)
	}

	if found, err := UpdateScreenshotNames(testSummariesPath, attachementDir); err != nil {
		log.Warnf("Failed to update screenshot names, error: %s", err)
	} else if !found {
		return nil
	}

	// deploy zipped attachments
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		return errors.New("no BITRISE_DEPLOY_DIR found")
	}

	zipedTestsDerivedDataPath := filepath.Join(deployDir, fmt.Sprintf("%s-xc-test-Attachments.zip", scheme))
	if err := cmd.Zip(filepath.Dir(attachementDir), filepath.Base(attachementDir), zipedTestsDerivedDataPath); err != nil {
		return err
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_ATTACHMENTS_PATH", zipedTestsDerivedDataPath); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_TEST_ATTACHMENTS_PATH, error: %s", err)
	}

	log.Donef("The zipped attachments are available in: %s", zipedTestsDerivedDataPath)
	return nil
}

func getSummariesAndAttachmentPath(testOutputDir string) (testSummariesPath string, attachmentDir string, err error) {
	const testSummaryFileName = "TestSummaries.plist"
	if exist, err := pathutil.IsDirExists(testOutputDir); err != nil {
		return "", "", err
	} else if !exist {
		return "", "", fmt.Errorf("no test logs found at: %s", testOutputDir)
	}

	testSummariesPath = path.Join(testOutputDir, testSummaryFileName)
	if exist, err := pathutil.IsPathExists(testSummariesPath); err != nil {
		return "", "", err
	} else if !exist {
		return "", "", fmt.Errorf("no test summaries found at: %s", testSummariesPath)
	}

	var attachementDir string
	{
		attachementDir = filepath.Join(testOutputDir, "Attachments")
		if exist, err := pathutil.IsDirExists(attachementDir); err != nil {
			return "", "", err
		} else if !exist {
			return "", "", fmt.Errorf("no test attachments found at: %s", attachementDir)
		}
	}

	log.Debugf("Test summaries path: %s", testSummariesPath)
	log.Debugf("Attachment dir: %s", attachementDir)
	return testSummariesPath, attachementDir, nil
}

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

//--------------------
// Main
//--------------------

func main() {
	var configs Configs
	if err := stepconf.Parse(&configs); err != nil {
		fail("Issue with input: %s", err)
	}

	stepconf.Print(configs)
	fmt.Println()
	log.SetEnableDebugLog(configs.Verbose)

	absProjectPath, err := pathutil.AbsPath(configs.ProjectPath)
	if err != nil {
		fail("Failed to get absolute project path, error: %s", err)
	}

	// Project-or-Workspace flag
	action := ""
	if strings.HasSuffix(absProjectPath, ".xcodeproj") {
		action = "-project"
	} else if strings.HasSuffix(absProjectPath, ".xcworkspace") {
		action = "-workspace"
	} else {
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			fmt.Println()
		}
		fail("Invalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", absProjectPath)
	}

	log.Printf("* action: %s", action)

	// Detect Xcode major version
	xcodebuildVersion, err := utility.GetXcodeVersion()
	if err != nil {
		fail("Failed to determine xcode version, error: %s", err)
	}
	log.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	if xcodebuildVersion.MajorVersion < 9 && configs.HeadlessMode {
		log.Warnf("Headless mode is enabled but it's only available with Xcode 9.x or newer.")
	}

	xcodeMajorVersion := xcodebuildVersion.MajorVersion
	if xcodeMajorVersion < minSupportedXcodeMajorVersion {
		fail("Invalid xcode major version (%d), should not be less then min supported: %d", xcodeMajorVersion, minSupportedXcodeMajorVersion)
	}

	if configs.ExportUITestArtifacts && xcodeMajorVersion >= 11 {
		// The test result bundle (xcresult) structure changed in Xcode 11:
		// it does not contains TestSummaries.plist nor Attachments directly.
		log.Warnf("Export UITest Artifacts (export_uitest_artifacts) turned on, but Xcode version >= 11. The test result bundle structure changed in Xcode 11 it does not contain TestSummaries.plist and Attachments directly, nothing to export.")
	}

	// Detect xcpretty version
	outputTool := configs.OutputTool
	xcprettyVersion, err := InstallXcpretty()
	if err != nil {
		log.Warnf("%s", err)
		log.Printf("Switching to xcodebuild for output tool")
		outputTool = "xcodebuild"
	} else {
		log.Printf("- xcprettyVersion: %s", xcprettyVersion.String())
		fmt.Println()
	}

	// Simulator infos
	var (
		sim             simulator.InfoModel
		osVersion       string
		errGetSimulator error
	)

	platform := strings.TrimSuffix(configs.SimulatorPlatform, " Simulator")
	if configs.SimulatorOsVersion == "latest" {
		var simulatorDevice = configs.SimulatorDevice
		if simulatorDevice == "iPad" {
			log.Warnf("Given device (%s) is deprecated, using (iPad 2)...", simulatorDevice)
			simulatorDevice = "iPad Air (3rd generation)"
		}

		sim, osVersion, errGetSimulator = simulator.GetLatestSimulatorInfoAndVersion(platform, simulatorDevice)
	} else {
		normalizedOsVersion := configs.SimulatorOsVersion
		osVersionSplit := strings.Split(normalizedOsVersion, ".")
		if len(osVersionSplit) > 2 {
			normalizedOsVersion = strings.Join(osVersionSplit[0:2], ".")
		}
		platformAndVersion := fmt.Sprintf("%s %s", platform, normalizedOsVersion)

		sim, errGetSimulator = simulator.GetSimulatorInfo(platformAndVersion, configs.SimulatorDevice)
	}

	if errGetSimulator != nil {
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		fail("failed to get simulator udid, error: %s", errGetSimulator)
	}

	log.Infof("Simulator infos")
	log.Printf("* simulator_name: %s, version: %s, UDID: %s, status: %s", sim.Name, osVersion, sim.ID, sim.Status)

	// Device Destination
	deviceDestination := fmt.Sprintf("id=%s", sim.ID)

	log.Printf("* device_destination: %s", deviceDestination)
	fmt.Println()

	// Create temporary directory for test outputs
	var testOutputDir string
	{
		tempDir, err := ioutil.TempDir("", "XCUITestOutput")
		if err != nil {
			fail("Could not create test output temporary directory.")
		}
		// Leaving the output dir in place after exiting
		testOutputDir = path.Join(tempDir, "Test.xcresult")
	}

	buildParams := models.XcodeBuildParamsModel{
		Action:                    action,
		ProjectPath:               absProjectPath,
		Scheme:                    configs.Scheme,
		DeviceDestination:         deviceDestination,
		CleanBuild:                configs.IsCleanBuild,
		DisableIndexWhileBuilding: configs.DisableIndexWhileBuilding,
	}

	buildTestParams := models.XcodeBuildTestParamsModel{
		BuildParams:          buildParams,
		TestOutputDir:        testOutputDir,
		BuildBeforeTest:      configs.ShouldBuildBeforeTest,
		AdditionalOptions:    configs.TestOptions,
		OnlyTestOptions:      "", // OnlyTestOptions will start empty as we expect to run all tests.
		GenerateCodeCoverage: configs.GenerateCodeCoverageFiles,
	}

	if configs.IsSingleBuild {
		buildTestParams.CleanBuild = configs.IsCleanBuild
	}

	//
	// If headless mode disabled - Start simulator
	if sim.Status == "Shutdown" && !configs.HeadlessMode {
		log.Infof("Booting simulator (%s)...", sim.ID)

		if err := simulator.BootSimulator(sim, xcodebuildVersion); err != nil {
			if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
				log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			}
			fail("failed to boot simulator, error: ", err)
		}

		progress.NewDefaultWrapper("Waiting for simulator boot").WrapAction(func() {
			time.Sleep(60 * time.Second)
		})

		fmt.Println()
	}

	//
	// Run build
	if !configs.IsSingleBuild {
		if rawXcodebuildOutput, exitCode, buildErr := runBuild(buildParams, outputTool); buildErr != nil {
			if _, err := saveRawOutputToLogFile(rawXcodebuildOutput, false); err != nil {
				log.Warnf("Failed to save the Raw Output, err: %s", err)
			}

			log.Warnf("xcode build exit code: %d", exitCode)
			log.Warnf("xcode build log:\n%s", rawXcodebuildOutput)
			log.Errorf("xcode build failed with error: %s", buildErr)
			if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
				log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			}
			os.Exit(1)
		}
	}

	var swiftPackagesPath string
	if xcodeMajorVersion >= 11 {
		var err error
		swiftPackagesPath, err = cache.SwiftPackagesPath(absProjectPath)
		if err != nil {
			fail("Failed to get Swift Packages path, error: %s", err)
		}
	}

	//
	// Run test
	rawXcodebuildOutput, exitCode, testErr := runTest(buildTestParams, outputTool, configs.XcprettyTestOptions, true, configs.ShouldRetryTestOnFail, configs.ShouldRetryTestOnFail, configs.TestFailureRetryLimit, 0, swiftPackagesPath)

	logPth, err := saveRawOutputToLogFile(rawXcodebuildOutput, (testErr == nil))

	if err != nil {
		log.Warnf("Failed to save the Raw Output, error: %s", err)
	}

	// exporting xcresult only if test result dir is present
	if addonResultPath := os.Getenv(bitriseConfigs.BitrisePerStepTestResultDirEnvKey); len(addonResultPath) > 0 {
		fmt.Println()
		log.Infof("Exporting test results")

		if err := copyAndSaveMetadata(addonCopy{
			sourceTestOutputDir:   buildTestParams.TestOutputDir,
			targetAddonPath:       addonResultPath,
			targetAddonBundleName: buildTestParams.BuildParams.Scheme,
		}); err != nil {
			log.Warnf("Failed to export test results, error: %s", err)
		}
	}

	if configs.ExportUITestArtifacts && xcodeMajorVersion < 11 {
		// The test result bundle (xcresult) structure changed in Xcode 11:
		// it does not contains TestSummaries.plist nor Attachments directly.
		fmt.Println()
		log.Infof("Exporting attachments")

		testSummariesPath, attachementDir, err := getSummariesAndAttachmentPath(buildTestParams.TestOutputDir)
		if err != nil {
			log.Warnf("Failed to export UI test artifacts, error: %s", err)
		}

		if err := saveAttachments(configs.Scheme, testSummariesPath, attachementDir); err != nil {
			log.Warnf("Failed to export UI test artifacts, error: %s", err)
		}
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCRESULT_PATH", buildTestParams.TestOutputDir); err != nil {
		log.Warnf("Failed to export: BITRISE_XCRESULT_PATH, error: %s", err)
	}

	if testErr != nil {
		log.Warnf("xcode test exit code: %d", exitCode)
		log.Errorf("xcode test failed, error: %s", testErr)
		log.Errorf("\nLast lines of the Xcode's build log:")
		fmt.Println(stringutil.LastNLines(rawXcodebuildOutput, 10))
		log.Warnf(`If you can't find the reason of the error in the log, please check the raw-xcodebuild-output.log
The log file is stored in $BITRISE_DEPLOY_DIR, and its full path
is available in the $BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH environment variable.

You can check the full, unfiltered and unformatted Xcode output in the file:
%s
If you have the Deploy to Bitrise.io step (after this step),
that will attach the file to your build as an artifact!`, logPth)
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	// Cache swift PM
	if xcodeMajorVersion >= 11 && configs.CacheLevel == "swift_packages" {
		if err := cache.CollectSwiftPackages(absProjectPath); err != nil {
			log.Warnf("Failed to mark swift packages for caching, error: %s", err)
		}
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded"); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}
}
