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

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	cmd "github.com/bitrise-io/steps-xcode-test/command"
	"github.com/bitrise-io/steps-xcode-test/models"
	"github.com/bitrise-io/steps-xcode-test/xcodeutil"
	shellquote "github.com/kballard/go-shellquote"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.

// This boot timeout can happen when running Unit Tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageIPhoneSimulator = "iPhoneSimulator: Timed out waiting"

// This boot timeout can happen when running Xcode (7+) UI tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageUITest = "Terminating app due to uncaught exception '_XCTestCaseInterruptionException'"

const earlyUnexpectedExit = "Early unexpected exit, operation never finished bootstrapping - no restart will be attempted"
const failureAttemptingToLaunch = "Assertion Failure: <unknown>:0: UI Testing Failure - Failure attempting to launch <XCUIApplicationImpl:"
const failedToBackgroundTestRunner = `Error Domain=IDETestOperationsObserverErrorDomain Code=12 "Failed to background test runner.`
const appStateIsStillNotRunning = `App state is still not running active, state = XCApplicationStateNotRunning`

var automaticRetryReasonPatterns = []string{
	timeOutMessageIPhoneSimulator,
	timeOutMessageUITest,
	earlyUnexpectedExit,
	failureAttemptingToLaunch,
	failedToBackgroundTestRunner,
	appStateIsStillNotRunning,
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
	TestOptions         string
	XcprettyTestOptions string
}

func (configs ConfigsModel) print() {
	fmt.Println()
	log.Info("Project Parameters:")
	log.Detail("- ProjectPath: %s", configs.ProjectPath)
	log.Detail("- Scheme: %s", configs.Scheme)

	fmt.Println()
	log.Info("Simulator Configs:")
	log.Detail("- SimulatorPlatform: %s", configs.SimulatorPlatform)
	log.Detail("- SimulatorDevice: %s", configs.SimulatorDevice)
	log.Detail("- SimulatorOsVersion: %s", configs.SimulatorOsVersion)

	fmt.Println()
	log.Info("Test Run Configs:")
	log.Detail("- OutputTool: %s", configs.OutputTool)
	log.Detail("- IsCleanBuild: %s", configs.IsCleanBuild)
	log.Detail("- IsSingleBuild: %s", configs.IsSingleBuild)

	log.Detail("- ShouldBuildBeforeTest: %s", configs.ShouldBuildBeforeTest)
	log.Detail("- ShouldRetryTestOnFail: %s", configs.ShouldRetryTestOnFail)

	log.Detail("- GenerateCodeCoverageFiles: %s", configs.GenerateCodeCoverageFiles)
	log.Detail("- ExportUITestArtifacts: %s", configs.ExportUITestArtifacts)

	log.Detail("- TestOptions: %s", configs.TestOptions)
	log.Detail("- XcprettyTestOptions: %s", configs.XcprettyTestOptions)
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
		TestOptions:         os.Getenv("xcodebuild_test_options"),
		XcprettyTestOptions: os.Getenv("xcpretty_test_options"),
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

	return nil
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
	validateRequiredInput(key, value)

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
		log.Warn("Failed to compile regexp: %s", err)
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

	log.Detail("$ %s", cmdArgsForPrint)

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

	log.Detail("$ set -o pipefail && %s | %v",
		cmd.PrintableCommandArgsWithEnvs(buildCmd.Args, xcodeCommandEnvs),
		cmd.PrintableCommandArgs(prettyCmd.Args))

	fmt.Println()

	if err := buildCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}
	if err := prettyCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}

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
	if err := pipeWriter.Close(); err != nil {
		return buildOutBuffer.String(), 1, err
	}

	if err := prettyCmd.Wait(); err != nil {
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

	log.Info("Building the project...")

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
				log.Warn("Automatic retry reason found in log: %s", retryReasonPattern)
				if isAutomaticRetryOnReason {
					log.Detail("isAutomaticRetryOnReason=true - retrying...")
					return runTest(buildTestParams, outputTool, xcprettyOptions, false, false)
				}
				log.Error("isAutomaticRetryOnReason=false, no more retry, stopping the test!")
				return fullOutputStr, exitCode, testError
			}
		}

		//
		// Retry on fail
		if isRetryOnFail {
			log.Warn("Test run failed")
			log.Detail("isRetryOnFail=true - retrying...")
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
			log.Warn("DEBUG: xcprettyOutputFilePath specifid")
			if isExist, err := pathutil.IsPathExists(xcprettyOutputFilePath); err != nil {
				log.Error("Failed to check xcpretty output file status (path: %s), error: %s", xcprettyOutputFilePath, err)
			} else if isExist {
				log.Warn("=> Deleting existing xcpretty output: %s", xcprettyOutputFilePath)
				if err := os.Remove(xcprettyOutputFilePath); err != nil {
					log.Error("Failed to delete xcpretty output file (path: %s), error: %s", xcprettyOutputFilePath, err)
				}
			} else {
				log.Warn("DEBUG: No xcprettyOutputFilePath exists")
			}
		}
		//
		xcprettyArgs = append(xcprettyArgs, options...)
	}

	log.Info("Running the tests...")

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

func saveRawOutputToLogFile(rawXcodebuildOutput string, isRunSuccess bool) error {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("xcodebuild-output")
	if err != nil {
		return fmt.Errorf("Failed to create temp dir, error: %s", err)
	}
	logFileName := "raw-xcodebuild-output.log"
	logPth := filepath.Join(tmpDir, logFileName)
	if err := fileutil.WriteStringToFile(logPth, rawXcodebuildOutput); err != nil {
		return fmt.Errorf("Failed to write xcodebuild output to file, error: %s", err)
	}

	if !isRunSuccess {
		deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
		if deployDir == "" {
			return errors.New("No BITRISE_DEPLOY_DIR found")
		}
		deployPth := filepath.Join(deployDir, logFileName)

		if err := cmdex.CopyFile(logPth, deployPth); err != nil {
			return fmt.Errorf("Failed to copy xcodebuild output log file from (%s) to (%s), error: %s", logPth, deployPth, err)
		}
		logPth = deployPth
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH", logPth); err != nil {
		log.Warn("Failed to export: BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH, error: %s", err)
	}
	return nil
}

func saveAttachements(projectPath, scheme string) error {
	projectName := filepath.Base(projectPath)
	projectExt := filepath.Ext(projectName)
	projectName = strings.TrimSuffix(projectName, projectExt)

	userHome := pathutil.UserHomeDir()
	deviedDataDir := filepath.Join(userHome, "Library/Developer/Xcode/DerivedData")
	projectDerivedDataDirPattern := filepath.Join(deviedDataDir, fmt.Sprintf("%s-*", projectName))
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

	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		return errors.New("No BITRISE_DEPLOY_DIR found")
	}

	zipedTestsDerivedDataPath := filepath.Join(deployDir, fmt.Sprintf("%s-xc-test-Attachments.zip", scheme))
	if err := cmd.Zip(testLogDir, "Attachments", zipedTestsDerivedDataPath); err != nil {
		return err
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_ATTACHMENTS_PATH", zipedTestsDerivedDataPath); err != nil {
		log.Warn("Failed to export: BITRISE_XCODE_TEST_ATTACHMENTS_PATH, error: %s", err)
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
		log.Error("Issue with input: %s", err)
		os.Exit(1)
	}

	fmt.Println()
	log.Info("Other Configs:")

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
		log.Error("Iinvalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", configs.ProjectPath)
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	log.Detail("* action: %s", action)

	// Device Destination
	deviceDestination := fmt.Sprintf("platform=%s,name=%s,OS=%s", configs.SimulatorPlatform, configs.SimulatorDevice, configs.SimulatorOsVersion)

	log.Detail("* device_destination: %s", deviceDestination)

	// Output tools versions
	xcodebuildVersion, err := xcodeutil.GetXcodeVersion()
	if err != nil {
		log.Error("Failed to get the version of xcodebuild! Error: %s", err)
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	log.Detail("* xcodebuild_version: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	xcprettyVersion, err := cmd.GetXcprettyVersion()
	if err != nil {
		log.Warn("Failed to get the xcpretty version! Error: %s", err)
	} else {
		log.Detail("* xcpretty_version: %s", xcprettyVersion)
	}

	// Simulator infos
	simulator, err := xcodeutil.GetSimulator(configs.SimulatorPlatform, configs.SimulatorDevice, configs.SimulatorOsVersion)
	if err != nil {
		log.Error(fmt.Sprintf("failed to get simulator udid, error: %s", err))
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	log.Detail("* simulator_name: %s, UDID: %s, status: %s", simulator.Name, simulator.SimID, simulator.Status)
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
		log.Info("Booting simulator (%s)...", simulator.SimID)

		if err := xcodeutil.BootSimulator(simulator, xcodebuildVersion); err != nil {
			log.Error(fmt.Sprintf("failed to boot simulator, error: %s", err))
			if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
				log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			}
			os.Exit(1)
		}

		fmt.Println()
	}

	//
	// Run build
	if !singleBuild {
		if rawXcodebuildOutput, exitCode, buildErr := runBuild(buildParams, configs.OutputTool); buildErr != nil {
			if err := saveRawOutputToLogFile(rawXcodebuildOutput, false); err != nil {
				log.Warn("Failed to save the Raw Output, err: %s", err)
			}

			log.Warn("xcode build exit code: %d", exitCode)
			log.Warn("xcode build log:\n%s", rawXcodebuildOutput)
			log.Error("xcode build failed with error: %s", buildErr)
			if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
				log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			}
			os.Exit(1)
		}
	}

	//
	// Run test
	rawXcodebuildOutput, exitCode, testErr := runTest(buildTestParams, configs.OutputTool, configs.XcprettyTestOptions, true, retryOnFail)

	if err := saveRawOutputToLogFile(rawXcodebuildOutput, (testErr == nil)); err != nil {
		log.Warn("Failed to save the Raw Output, error %s", err)
	}

	if exportUITestArtifacts {
		if err := saveAttachements(configs.ProjectPath, configs.Scheme); err != nil {
			log.Warn("Failed to export UI test artifacts, error %s", err)
		}
	}

	if testErr != nil {
		log.Warn("xcode test exit code: %d", exitCode)
		log.Error("xcode test failed, error: %s", testErr)
		hint := `If you can't find the reason of the error in the log, please check the raw-xcodebuild-output.log
The log file is stored in $BITRISE_DEPLOY_DIR, and its full path
is available in the $BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH environment variable`
		log.Warn(hint)
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		os.Exit(1)
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded"); err != nil {
		log.Warn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}
}
