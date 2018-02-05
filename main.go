package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/stringutil"
	cmd "github.com/bitrise-io/steps-xcode-test/command"
	"github.com/bitrise-io/steps-xcode-test/models"
	"github.com/bitrise-io/steps-xcode-test/xcodeutil"
	shellquote "github.com/kballard/go-shellquote"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.

const (
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

// ConfigsModel ...
type ConfigsModel struct {
	// Project Parameters
	ProjectPath string
	Scheme      string

	// Simulator Configs
	SimulatorPlatform  string
	SimulatorDevice    string
	SimulatorOsVersion string

	// Test Run Configs
	OutputTool    string
	IsCleanBuild  string
	IsSingleBuild string

	ShouldBuildBeforeTest string
	ShouldRetryTestOnFail string

	GenerateCodeCoverageFiles string
	ExportUITestArtifacts     string

	// Not required parameters
	TestOptions          string
	XcprettyTestOptions  string
	WaitForSimulatorBoot string
}

func (configs ConfigsModel) print() {
	fmt.Println()
	log.Infof("Project Parameters:")
	log.Printf("- ProjectPath: %s", configs.ProjectPath)
	log.Printf("- Scheme: %s", configs.Scheme)

	fmt.Println()
	log.Infof("Simulator Configs:")
	log.Printf("- SimulatorPlatform: %s", configs.SimulatorPlatform)
	log.Printf("- SimulatorDevice: %s", configs.SimulatorDevice)
	log.Printf("- SimulatorOsVersion: %s", configs.SimulatorOsVersion)

	fmt.Println()
	log.Infof("Test Run Configs:")
	log.Printf("- OutputTool: %s", configs.OutputTool)
	log.Printf("- IsCleanBuild: %s", configs.IsCleanBuild)
	log.Printf("- IsSingleBuild: %s", configs.IsSingleBuild)

	log.Printf("- ShouldBuildBeforeTest: %s", configs.ShouldBuildBeforeTest)
	log.Printf("- ShouldRetryTestOnFail: %s", configs.ShouldRetryTestOnFail)

	log.Printf("- GenerateCodeCoverageFiles: %s", configs.GenerateCodeCoverageFiles)
	log.Printf("- ExportUITestArtifacts: %s", configs.ExportUITestArtifacts)

	log.Printf("- TestOptions: %s", configs.TestOptions)
	log.Printf("- XcprettyTestOptions: %s", configs.XcprettyTestOptions)
	log.Printf("- WaitForSimulatorBoot: %s", configs.WaitForSimulatorBoot)
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		// Project Parameters
		ProjectPath: os.Getenv("project_path"),
		Scheme:      os.Getenv("scheme"),

		// Simulator Configs
		SimulatorPlatform:  os.Getenv("simulator_platform"),
		SimulatorDevice:    os.Getenv("simulator_device"),
		SimulatorOsVersion: os.Getenv("simulator_os_version"),

		// Test Run Configs
		OutputTool:    os.Getenv("output_tool"),
		IsCleanBuild:  os.Getenv("is_clean_build"),
		IsSingleBuild: os.Getenv("single_build"),

		ShouldBuildBeforeTest: os.Getenv("should_build_before_test"),
		ShouldRetryTestOnFail: os.Getenv("should_retry_test_on_fail"),

		GenerateCodeCoverageFiles: os.Getenv("generate_code_coverage_files"),
		ExportUITestArtifacts:     os.Getenv("export_uitest_artifacts"),

		// Not required parameters
		TestOptions:          os.Getenv("xcodebuild_test_options"),
		XcprettyTestOptions:  os.Getenv("xcpretty_test_options"),
		WaitForSimulatorBoot: os.Getenv("wait_for_simulator_boot"),
	}
}

func (configs ConfigsModel) validate() error {
	// required
	if err := validateRequiredInput(configs.ProjectPath, "project_path"); err != nil {
		return err
	}
	if err := validateRequiredInput(configs.Scheme, "scheme"); err != nil {
		return err
	}

	if err := validateRequiredInput(configs.SimulatorPlatform, "simulator_platform"); err != nil {
		return err
	}
	if err := validateRequiredInput(configs.SimulatorDevice, "simulator_device"); err != nil {
		return err
	}
	if err := validateRequiredInput(configs.SimulatorOsVersion, "simulator_os_version"); err != nil {
		return err
	}

	if err := validateRequiredInputWithOptions(configs.OutputTool, "output_tool", []string{"xcpretty", "xcodebuild"}); err != nil {
		return err
	}
	if err := validateRequiredInputWithOptions(configs.IsCleanBuild, "is_clean_build", []string{"yes", "no"}); err != nil {
		return err
	}
	if err := validateRequiredInputWithOptions(configs.IsSingleBuild, "single_build", []string{"true", "false"}); err != nil {
		return err
	}

	if err := validateRequiredInputWithOptions(configs.ShouldBuildBeforeTest, "should_build_before_test", []string{"yes", "no"}); err != nil {
		return err
	}
	if err := validateRequiredInputWithOptions(configs.ShouldRetryTestOnFail, "should_retry_test_on_fail", []string{"yes", "no"}); err != nil {
		return err
	}

	if err := validateRequiredInputWithOptions(configs.GenerateCodeCoverageFiles, "generate_code_coverage_files", []string{"yes", "no"}); err != nil {
		return err
	}
	if err := validateRequiredInputWithOptions(configs.ExportUITestArtifacts, "export_uitest_artifacts", []string{"true", "false"}); err != nil {
		return err
	}
	return validateRequiredInputWithOptions(configs.WaitForSimulatorBoot, "wait_for_simulator_boot", []string{"yes", "no"})
}

//--------------------
// Functions
//--------------------

func validateRequiredInput(value, key string) error {
	if value == "" {
		return fmt.Errorf("Missing required input: %s", key)
	}
	return nil
}

func validateRequiredInputWithOptions(value, key string, options []string) error {
	if err := validateRequiredInput(key, value); err != nil {
		return err
	}

	found := false
	for _, option := range options {
		if option == value {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Invalid input: (%s) value: (%s), valid options: %s", key, value, strings.Join(options, ", "))
	}

	return nil
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
				return outBuffer.String(), 1, errors.New("Failed to cast exit status")
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
				return buildOutBuffer.String(), 1, errors.New("Failed to cast exit status")
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
	xcodebuildArgs = append(xcodebuildArgs, "build", "-destination", buildParams.DeviceDestination)

	log.Infof("Building the project...")

	if outputTool == "xcpretty" {
		return runPrettyXcodeBuildCmd(false, []string{}, xcodebuildArgs)
	}
	return runXcodeBuildCmd(false, xcodebuildArgs...)
}

func runTest(buildTestParams models.XcodeBuildTestParamsModel, outputTool, xcprettyOptions string, isAutomaticRetryOnReason, isRetryOnFail bool) (string, int, error) {
	handleTestError := func(fullOutputStr string, exitCode int, testError error) (string, int, error) {
		//
		// Automatic retry
		for _, retryReasonPattern := range automaticRetryReasonPatterns {
			if isStringFoundInOutput(retryReasonPattern, fullOutputStr) {
				log.Warnf("Automatic retry reason found in log: %s", retryReasonPattern)
				if isAutomaticRetryOnReason {
					log.Printf("isAutomaticRetryOnReason=true - retrying...")
					return runTest(buildTestParams, outputTool, xcprettyOptions, false, false)
				}
				log.Errorf("isAutomaticRetryOnReason=false, no more retry, stopping the test!")
				return fullOutputStr, exitCode, testError
			}
		}

		//
		// Retry on fail
		if isRetryOnFail {
			log.Warnf("Test run failed")
			log.Printf("isRetryOnFail=true - retrying...")
			return runTest(buildTestParams, outputTool, xcprettyOptions, false, false)
		}

		return fullOutputStr, exitCode, testError
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
	// Related issue link: https://github.com/bitrise-io/steps-xcode-test/issues/55
	if buildTestParams.BuildBeforeTest {
		xcodebuildArgs = append(xcodebuildArgs, "build")
	}
	xcodebuildArgs = append(xcodebuildArgs, "test", "-destination", buildParams.DeviceDestination)

	if buildTestParams.GenerateCodeCoverage {
		xcodebuildArgs = append(xcodebuildArgs, "GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES")
		xcodebuildArgs = append(xcodebuildArgs, "GCC_GENERATE_TEST_COVERAGE_FILES=YES")
	}

	if buildTestParams.AdditionalOptions != "" {
		options, err := shellquote.Split(buildTestParams.AdditionalOptions)
		if err != nil {
			return "", 1, fmt.Errorf("failed to parse additional options (%s), error: %s", buildTestParams.AdditionalOptions, err)
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

func saveRawOutputToLogFile(rawXcodebuildOutput string, isRunSuccess bool) (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("xcodebuild-output")
	if err != nil {
		return "", fmt.Errorf("Failed to create temp dir, error: %s", err)
	}
	logFileName := "raw-xcodebuild-output.log"
	logPth := filepath.Join(tmpDir, logFileName)
	if err := fileutil.WriteStringToFile(logPth, rawXcodebuildOutput); err != nil {
		return "", fmt.Errorf("Failed to write xcodebuild output to file, error: %s", err)
	}

	if !isRunSuccess {
		deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
		if deployDir == "" {
			return "", errors.New("No BITRISE_DEPLOY_DIR found")
		}
		deployPth := filepath.Join(deployDir, logFileName)

		if err := command.CopyFile(logPth, deployPth); err != nil {
			return "", fmt.Errorf("Failed to copy xcodebuild output log file from (%s) to (%s), error: %s", logPth, deployPth, err)
		}
		logPth = deployPth
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH", logPth); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH, error: %s", err)
	}
	return logPth, nil
}

func screenshotName(startTime time.Time, title, uuid string) string {
	formattedDate := startTime.Format("2006-01-02_03-04-05")
	fixedTitle := strings.Replace(title, " ", "_", -1)
	return fmt.Sprintf("%s_%s_%s", formattedDate, fixedTitle, uuid)
}

func updateScreenshotNames(testLogsDir string) error {
	testSummariesPattern := filepath.Join(testLogsDir, "*_TestSummaries.plist")
	testSummariesPths, err := filepath.Glob(testSummariesPattern)
	if err != nil {
		return err
	} else if len(testSummariesPths) == 0 {
		return fmt.Errorf("no TestSummaries file found with pattern: %s", testSummariesPattern)
	}
	testSummariesPth := testSummariesPths[0]

	testSummariesContent, err := fileutil.ReadStringFromFile(testSummariesPth)
	if err != nil {
		return err
	}

	testItems, err := xcodeutil.CollectTestItemsWithScreenshot(testSummariesContent)
	if err != nil {
		return err
	}

	for _, testItem := range testItems {
		startTimeIntervalObj, found := testItem["StartTimeInterval"]
		if !found {
			return fmt.Errorf("missing StartTimeInterval")
		}
		startTimeInterval, casted := startTimeIntervalObj.(float64)
		if !casted {
			return fmt.Errorf("StartTimeInterval is not a float64")
		}
		startTime, err := xcodeutil.TimestampToTime(startTimeInterval)
		if err != nil {
			return err
		}

		titleObj, found := testItem["Title"]
		if !found {
			return fmt.Errorf("missing Title")
		}
		title, casted := titleObj.(string)
		if !casted {
			return fmt.Errorf("Title is not a string")
		}

		uuidObj, found := testItem["UUID"]
		if !found {
			return fmt.Errorf("missing UUID")
		}
		uuid, casted := uuidObj.(string)
		if !casted {
			return fmt.Errorf("UUID is not a string")
		}

		origScreenshotPth := filepath.Join(testLogsDir, "Attachments", fmt.Sprintf("Screenshot_%s.png", uuid))
		if exist, err := pathutil.IsPathExists(origScreenshotPth); err != nil {
			return err
		} else if exist {
			newScreenshotPth := filepath.Join(testLogsDir, "Attachments", screenshotName(startTime, title, uuid)+".png")
			if err := os.Rename(origScreenshotPth, newScreenshotPth); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("screenshot not exists at: %s", origScreenshotPth)
		}
	}

	return nil
}

func saveAttachments(projectPath, scheme string) error {
	// find project derived data
	projectName := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))

	// change spaces to _
	projectName = strings.Replace(projectName, " ", "_", -1)

	userHome := pathutil.UserHomeDir()
	derivedDataDir := filepath.Join(userHome, "Library/Developer/Xcode/DerivedData")

	projectDerivedDataDirPattern := filepath.Join(derivedDataDir, fmt.Sprintf("%s-*", projectName))
	projectDerivedDataDirs, err := filepath.Glob(projectDerivedDataDirPattern)
	if err != nil {
		return err
	}

	if len(projectDerivedDataDirs) > 1 {
		return fmt.Errorf("more than 1 project derived data dir found: %v, with pattern: %s", projectDerivedDataDirs, projectDerivedDataDirPattern)
	} else if len(projectDerivedDataDirs) == 0 {
		return fmt.Errorf("no project derived data dir found with pattern: %s", projectDerivedDataDirPattern)
	}
	projectDerivedDataDir := projectDerivedDataDirs[0]

	testLogDir := filepath.Join(projectDerivedDataDir, "Logs", "Test")
	if exist, err := pathutil.IsDirExists(testLogDir); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("no test logs found at: %s", projectDerivedDataDir)
	}

	testLogAttachmentsDir := filepath.Join(testLogDir, "Attachments")
	if exist, err := pathutil.IsDirExists(testLogAttachmentsDir); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("no test attachments found at: %s", testLogAttachmentsDir)
	}

	// update screenshot name: Screenshot_uuid.png -> start_date_time_title_uuid.png
	if err := updateScreenshotNames(testLogDir); err != nil {
		log.Warnf("Failed to update screenshot names, error: %s", err)
	}

	// deploy zipped attachments
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		return errors.New("No BITRISE_DEPLOY_DIR found")
	}

	zipedTestsDerivedDataPath := filepath.Join(deployDir, fmt.Sprintf("%s-xc-test-Attachments.zip", scheme))
	if err := cmd.Zip(testLogDir, "Attachments", zipedTestsDerivedDataPath); err != nil {
		return err
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_ATTACHMENTS_PATH", zipedTestsDerivedDataPath); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_TEST_ATTACHMENTS_PATH, error: %s", err)
	}

	return nil
}

//--------------------
// Main
//--------------------

func main() {
	configs := createConfigsModelFromEnvs()
	configs.print()
	if err := configs.validate(); err != nil {
		log.Errorf("Issue with input: %s", err)
		os.Exit(1)
	}

	fmt.Println()
	log.Infof("Other Configs:")

	cleanBuild := (configs.IsCleanBuild == "yes")
	generateCodeCoverage := (configs.GenerateCodeCoverageFiles == "yes")
	exportUITestArtifacts := (configs.ExportUITestArtifacts == "true")
	singleBuild := (configs.IsSingleBuild == "true")
	buildBeforeTest := (configs.ShouldBuildBeforeTest == "yes")
	retryOnFail := (configs.ShouldRetryTestOnFail == "yes")

	// Project-or-Workspace flag
	action := ""
	if strings.HasSuffix(configs.ProjectPath, ".xcodeproj") {
		action = "-project"
	} else if strings.HasSuffix(configs.ProjectPath, ".xcworkspace") {
		action = "-workspace"
	} else {
		log.Errorf("Invalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", configs.ProjectPath)
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	log.Printf("* action: %s", action)

	// Output tools versions
	xcodebuildVersion, err := xcodeutil.GetXcodeVersion()
	if err != nil {
		log.Errorf("Failed to get the version of xcodebuild! Error: %s", err)
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	log.Printf("* xcodebuild_version: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	xcprettyVersion, err := cmd.GetXcprettyVersion()
	if err != nil {
		log.Warnf("Failed to get the xcpretty version! Error: %s", err)
	} else {
		log.Printf("* xcpretty_version: %s", xcprettyVersion)
	}

	// Simulator infos
	simulator, err := xcodeutil.GetSimulator(configs.SimulatorPlatform, configs.SimulatorDevice, configs.SimulatorOsVersion)
	if err != nil {
		log.Errorf(fmt.Sprintf("failed to get simulator udid, error: %s", err))
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	log.Printf("* simulator_name: %s, UDID: %s, status: %s", simulator.Name, simulator.SimID, simulator.Status)

	// Device Destination
	deviceDestination := fmt.Sprintf("id=%s", simulator.SimID)

	log.Printf("* device_destination: %s", deviceDestination)
	fmt.Println()

	buildParams := models.XcodeBuildParamsModel{
		Action:            action,
		ProjectPath:       configs.ProjectPath,
		Scheme:            configs.Scheme,
		DeviceDestination: deviceDestination,
		CleanBuild:        cleanBuild,
	}

	buildTestParams := models.XcodeBuildTestParamsModel{
		BuildParams: buildParams,

		BuildBeforeTest:      buildBeforeTest,
		AdditionalOptions:    configs.TestOptions,
		GenerateCodeCoverage: generateCodeCoverage,
	}

	if singleBuild {
		buildTestParams.CleanBuild = cleanBuild
	}

	//
	// Start simulator
	if simulator.Status == "Shutdown" {
		log.Infof("Booting simulator (%s)...", simulator.SimID)

		if err := xcodeutil.BootSimulator(simulator, xcodebuildVersion); err != nil {
			log.Errorf(fmt.Sprintf("failed to boot simulator, error: %s", err))
			if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
				log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			}
			os.Exit(1)
		}

		if configs.WaitForSimulatorBoot == "yes" {
			progress.NewDefaultWrapper("Waiting for simulator boot").WrapAction(func() {
				time.Sleep(60 * time.Second)
			})
		}

		fmt.Println()
	}

	//
	// Run build
	if !singleBuild {
		if rawXcodebuildOutput, exitCode, buildErr := runBuild(buildParams, configs.OutputTool); buildErr != nil {
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

	//
	// Run test
	rawXcodebuildOutput, exitCode, testErr := runTest(buildTestParams, configs.OutputTool, configs.XcprettyTestOptions, true, retryOnFail)

	logPth, err := saveRawOutputToLogFile(rawXcodebuildOutput, (testErr == nil))

	if err != nil {
		log.Warnf("Failed to save the Raw Output, error %s", err)
	}

	if exportUITestArtifacts {
		if err := saveAttachments(configs.ProjectPath, configs.Scheme); err != nil {
			log.Warnf("Failed to export UI test artifacts, error %s", err)
		}
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

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded"); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}
}
