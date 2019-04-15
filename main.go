package main

import (
	"bytes"
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
	"github.com/bitrise-io/go-xcode/utility"
	cmd "github.com/bitrise-steplib/steps-xcode-test/command"
	"github.com/bitrise-steplib/steps-xcode-test/models"
	"github.com/bitrise-steplib/steps-xcode-test/xcodeutil"
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

	ShouldBuildBeforeTest bool `env:"should_build_before_test,opt[yes,no]"`
	ShouldRetryTestOnFail bool `env:"should_retry_test_on_fail,opt[yes,no]"`

	GenerateCodeCoverageFiles bool `env:"generate_code_coverage_files,opt[yes,no]"`
	ExportUITestArtifacts     bool `env:"export_uitest_artifacts,opt[true,false]"`

	DisableIndexWhileBuilding bool `env:"disable_index_while_building,opt[yes,no]"`

	// Not required parameters
	TestOptions         string `env:"xcodebuild_test_options"`
	XcprettyTestOptions string `env:"xcpretty_test_options"`

	// Debug
	Verbose      bool `env:"verbose,opt[yes,no]"`
	HeadlessMode bool `env:"headless_mode,opt[yes,no]"`
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
	}

	// Disable indexing during the build.
	// Indexing is needed for autocomplete, ability to quickly jump to definition, get class and method help by alt clicking.
	// Which are not needed in CI environment.
	if buildParams.DisableIndexWhileBuilding {
		xcodebuildArgs = append(xcodebuildArgs, "COMPILER_INDEX_STORE_ENABLE=NO")
	}

	xcodebuildArgs = append(xcodebuildArgs, "test", "-destination", buildParams.DeviceDestination)
	xcodebuildArgs = append(xcodebuildArgs, "-resultBundlePath", buildTestParams.TestOutputDir)

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

	// Project-or-Workspace flag
	action := ""
	if strings.HasSuffix(configs.ProjectPath, ".xcodeproj") {
		action = "-project"
	} else if strings.HasSuffix(configs.ProjectPath, ".xcworkspace") {
		action = "-workspace"
	} else {
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			fmt.Println()
		}
		fail("Invalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", configs.ProjectPath)
	}

	log.Printf("* action: %s", action)

	// Detect Xcode major version
	xcodebuildVersion, err := utility.GetXcodeVersion()
	if err != nil {
		fail("Failed to determine xcode version, error: %s", err)
	}
	log.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	if xcodebuildVersion.MajorVersion < 9 && configs.HeadlessMode {
		log.Warnf("Headless mode is enabled but it's only availabe with Xcode 9.x or newer.")
	}

	xcodeMajorVersion := xcodebuildVersion.MajorVersion
	if xcodeMajorVersion < minSupportedXcodeMajorVersion {
		fail("Invalid xcode major version (%d), should not be less then min supported: %d", xcodeMajorVersion, minSupportedXcodeMajorVersion)
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
	simulator, err := xcodeutil.GetSimulator(configs.SimulatorPlatform, configs.SimulatorDevice, configs.SimulatorOsVersion)
	if err != nil {
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		}
		fail("failed to get simulator udid, error: ", err)
	}

	log.Infof("Simulator infos")
	log.Printf("* simulator_name: %s, UDID: %s, status: %s", simulator.Name, simulator.SimID, simulator.Status)

	// Device Destination
	deviceDestination := fmt.Sprintf("id=%s", simulator.SimID)

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
		ProjectPath:               configs.ProjectPath,
		Scheme:                    configs.Scheme,
		DeviceDestination:         deviceDestination,
		CleanBuild:                configs.IsCleanBuild,
		DisableIndexWhileBuilding: configs.DisableIndexWhileBuilding,
	}

	buildTestParams := models.XcodeBuildTestParamsModel{
		BuildParams: buildParams,

		TestOutputDir:        testOutputDir,
		BuildBeforeTest:      configs.ShouldBuildBeforeTest,
		AdditionalOptions:    configs.TestOptions,
		GenerateCodeCoverage: configs.GenerateCodeCoverageFiles,
	}

	if configs.IsSingleBuild {
		buildTestParams.CleanBuild = configs.IsCleanBuild
	}

	//
	// If headless mode disabled - Start simulator
	if simulator.Status == "Shutdown" && !configs.HeadlessMode {
		log.Infof("Booting simulator (%s)...", simulator.SimID)

		if err := xcodeutil.BootSimulator(simulator, xcodebuildVersion); err != nil {
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

	//
	// Run test
	rawXcodebuildOutput, exitCode, testErr := runTest(buildTestParams, outputTool, configs.XcprettyTestOptions, true, configs.ShouldRetryTestOnFail)

	logPth, err := saveRawOutputToLogFile(rawXcodebuildOutput, (testErr == nil))

	if err != nil {
		log.Warnf("Failed to save the Raw Output, error: %s", err)
	}

	// exporting xcresult only if test result dir is present
	if testResultPath := os.Getenv(bitriseConfigs.BitriseTestResultDirEnvKey); len(testResultPath) > 0 {
		fmt.Println()
		log.Infof("Exporting test results")

		// the leading `/` means to copy not the content but the whole dir
		// -a means a better recursive, with symlinks handling and everything
		cmd := command.New("cp", "-a", buildTestParams.TestOutputDir, testResultPath+"/")

		log.Donef("$ %s", cmd.PrintableCommandArgs())

		if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
			log.Warnf("Failed to export test results, error: %s, output: %s", err, out)
		}
	}

	if configs.ExportUITestArtifacts {
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

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded"); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}
}
