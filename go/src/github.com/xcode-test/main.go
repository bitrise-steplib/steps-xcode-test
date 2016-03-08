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

	cmd "github.com/xcode-test/command"
	log "github.com/xcode-test/logutil"
	"github.com/xcode-test/xcodeutil"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.

// This boot timeout can happen when running Unit Tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageIPhoneSimulator = "iPhoneSimulator: Timed out waiting"

// This boot timeout can happen when running Xcode (7+) UI tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageUITest = "Terminating app due to uncaught exception '_XCTestCaseInterruptionException'"

func validateRequiredInput(value, key string) {
	if value == "" {
		log.LogFail("Missing required input: %s", key)
	}
}

func validateRequiredInputWithOptions(value, key string, options []string) {
	if value == "" {
		log.LogFail("Missing required input: %s", key)
	}
	found := false
	for _, option := range options {
		if option == value {
			found = true
			break
		}
	}

	if !found {
		log.LogFail("Invalid input: (%s) value: (%s), valid options: %s", key, value, strings.Join(options, ", "))
	}
}

func getXcprettyVersion() (string, error) {
	cmd := exec.Command("xcpretty", "-version")
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)
	if strings.HasSuffix(outStr, "\n") {
		outStr = strings.TrimSuffix(outStr, "\n")
	}

	if err != nil {
		return "", fmt.Errorf("xcpretty -version failed, err: %s, details: %s", err, outStr)
	}

	return outStr, nil
}

func isStringFoundInOutput(searchStr, outputToSearchIn string) bool {
	r, err := regexp.Compile("(?i)" + searchStr)
	if err != nil {
		log.LogWarn("Failed to compile regexp: %s", err)
		return false
	}
	return r.MatchString(outputToSearchIn)
}

// runXcodeBuildCmd ...
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

	cmdArgsForPrint := cmd.PrintableCommandArgs(buildCmd.Args)

	log.LogDetails("$ %s", cmdArgsForPrint)

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

// runPrettyXcodeBuildCmd ...
func runPrettyXcodeBuildCmd(useStdOut bool,
	testResultsFilePath string,
	args ...string) (string, int, error) {

	//
	buildCmd := cmd.CreateXcodebuildCmd(args...)
	prettyCmd := cmd.CreateXcprettyCmd(testResultsFilePath)
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

	log.LogDetails("$ set -o pipefail && %s | %v",
		cmd.PrintableCommandArgs(buildCmd.Args),
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

// runBuild ...
func runBuild(outputTool, action,
	projectPath, scheme string, cleanBuild bool,
	deviceDestination, derivedDataDir string) (string, int, error) {

	args := []string{action, projectPath, "-scheme", scheme}
	if cleanBuild {
		args = append(args, "clean")
	}
	args = append(args, "build", "-destination", deviceDestination, "-derivedDataPath", derivedDataDir)

	log.LogInfo("Building the project...")

	if outputTool == "xcpretty" {
		return runPrettyXcodeBuildCmd(false, "", args...)
	}
	return runXcodeBuildCmd(false, args...)
}

func runTest(outputTool, action, projectPath, scheme string,
	deviceDestination string, generateCodeCoverage bool,
	isRetryOnTimeout bool,
	testResultsFilePath, derivedDataDir string) (string, int, error) {

	handleTestError := func(fullOutputStr string, exitCode int, testError error) (string, int, error) {
		// fmt.Printf("\n\nfullOutputStr:\n\n%s", fullOutputStr)
		if isStringFoundInOutput(timeOutMessageIPhoneSimulator, fullOutputStr) {
			log.LogInfo("Simulator Timeout detected")
			if isRetryOnTimeout {
				log.LogDetails("isRetryOnTimeout=true - retrying...")
				return runTest(outputTool, action,
					projectPath, scheme, deviceDestination, generateCodeCoverage,
					false, testResultsFilePath, derivedDataDir)
			}
			log.LogWarn("isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, exitCode, testError
		}

		if isStringFoundInOutput(timeOutMessageUITest, fullOutputStr) {
			log.LogInfo("Simulator Timeout detected: isUITestTimeoutFound")
			if isRetryOnTimeout {
				log.LogDetails("isRetryOnTimeout=true - retrying...")
				return runTest(outputTool, action,
					projectPath, scheme, deviceDestination, generateCodeCoverage,
					false, testResultsFilePath, derivedDataDir)
			}
			log.LogWarn("isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, exitCode, testError
		}

		return fullOutputStr, exitCode, testError
	}

	args := []string{action, projectPath, "-scheme", scheme}
	// the 'build' argument is required *before* the 'test' arg, to prevent
	//  the Xcode bug described in the README, which causes:
	// 'iPhoneSimulator: Timed out waiting 120 seconds for simulator to boot, current state is 1.'
	//  in case the compilation takes a long time.
	// Related Radar link: https://openradar.appspot.com/22413115
	// Demonstration project: https://github.com/bitrise-io/simulator-launch-timeout-includes-build-time
	args = append(args, "build", "test", "-destination", deviceDestination, "-derivedDataPath", derivedDataDir)

	if generateCodeCoverage {
		args = append(args, "GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES")
		args = append(args, "GCC_GENERATE_TEST_COVERAGE_FILES=YES")
	}

	log.LogInfo("Running the tests...")

	var rawOutput string
	var err error
	var exit int
	if outputTool == "xcpretty" {
		rawOutput, exit, err = runPrettyXcodeBuildCmd(true, testResultsFilePath, args...)
	} else {
		rawOutput, exit, err = runXcodeBuildCmd(true, args...)
	}

	if err != nil {
		return handleTestError(rawOutput, exit, err)
	}
	return rawOutput, exit, nil
}

func zip(targetDir, targetRelPathToZip, zipPath string) error {
	zipCmd := exec.Command("/usr/bin/zip", "-rTy", zipPath, targetRelPathToZip)
	zipCmd.Dir = targetDir
	if out, err := zipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Zip failed, out: %s, err: %#v", out, err)
	}
	return nil
}

func saveRawOutputToLogFile(rawXcodebuildOutput string, isRunSuccess bool) error {
	outputFile, err := ioutil.TempFile(os.TempDir(), "temp")
	if err != nil {
		return fmt.Errorf("saveRawOutputToLogFile: failed to create Raw Output file: %s", err)
	}
	outputFilePath := outputFile.Name()

	defer func() {
		if err := outputFile.Close(); err != nil {
			log.LogWarn("Failed to close file:", err)
		}
	}()

	if _, err := outputFile.Write([]byte(rawXcodebuildOutput)); err != nil {
		return fmt.Errorf("saveRawOutputToLogFile: failed to write into the Raw Output file: %s", err)
	}

	if !isRunSuccess {
		deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
		if deployDir == "" {
			return errors.New("No BITRISE_DEPLOY_DIR found")
		}

		rawXcodebuildOutputDir := filepath.Dir(outputFilePath)
		rawXcodebuildOutputName := filepath.Base(outputFilePath)
		outputFilePath = path.Join(deployDir, "raw-xcodebuild-output.zip")
		if err := zip(rawXcodebuildOutputDir, rawXcodebuildOutputName, outputFilePath); err != nil {
			return err
		}
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH", outputFilePath); err != nil {
		log.LogWarn("Failed to export: BITRISE_XCODE_RAW_TEST_RESULT_TEXT_PATH, error: %s", err)
	}
	return nil
}

func saveAttachements(deviedDataPath string) error {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		return errors.New("No BITRISE_DEPLOY_DIR found")
	}

	zipedTestsDerivedDataPath := path.Join(deployDir, "attachments.zip")
	testsDerivedDataDir := path.Join(deviedDataPath, "Logs", "Test")
	if err := zip(testsDerivedDataDir, "Attachments", zipedTestsDerivedDataPath); err != nil {
		return err
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_ATTACHMENTS_PATH", zipedTestsDerivedDataPath); err != nil {
		log.LogWarn("Failed to export: BITRISE_XCODE_TEST_ATTACHMENTS_PATH, error: %s", err)
	}
	return nil
}

func main() {
	//
	// Required parameters
	projectPath := os.Getenv("project_path")
	scheme := os.Getenv("scheme")
	simulatorPlatform := os.Getenv("simulator_platform")
	simulatorDevice := os.Getenv("simulator_device")
	simulatorOsVersion := os.Getenv("simulator_os_version")
	testResultsFilePath := os.Getenv("test_results_file_path")

	//
	// Not required parameters
	isCleanBuild := os.Getenv("is_clean_build")
	generateCodeCoverageFiles := os.Getenv("generate_code_coverage_files")
	outputTool := os.Getenv("output_tool")
	exportUITestArtifactsStr := os.Getenv("export_uitest_artifacts")

	log.LogConfigs(
		projectPath,
		scheme,
		simulatorPlatform,
		simulatorDevice,
		simulatorOsVersion,
		testResultsFilePath,
		isCleanBuild,
		generateCodeCoverageFiles,
		outputTool,
		exportUITestArtifactsStr)

	validateRequiredInput(projectPath, "project_path")
	validateRequiredInput(scheme, "scheme")
	validateRequiredInput(simulatorPlatform, "simulator_platform")
	validateRequiredInput(simulatorDevice, "simulator_device")
	validateRequiredInput(simulatorOsVersion, "simulator_os_version")
	validateRequiredInput(testResultsFilePath, "test_results_file_path")
	validateRequiredInputWithOptions(outputTool, "output_tool", []string{"xcpretty", "xcodebuild"})

	cleanBuild := (isCleanBuild == "yes")
	generateCodeCoverage := (generateCodeCoverageFiles == "yes")
	exportUITestArtifacts := (exportUITestArtifactsStr == "true")

	fmt.Println()

	//
	// Project-or-Workspace flag
	action := ""
	if strings.HasSuffix(projectPath, ".xcodeproj") {
		action = "-project"
	} else if strings.HasSuffix(projectPath, ".xcworkspace") {
		action = "-workspace"
	} else {
		log.LogFail("Failed to get valid project file (invalid project file): %s", projectPath)
	}

	log.LogDetails("* action: %s", action)

	//
	// Device Destination
	// xcodebuild -project ./BitriseSampleWithYML.xcodeproj -scheme BitriseSampleWithYML  test -destination "platform=iOS Simulator,name=iPhone 6 Plus,OS=latest" -sdk iphonesimulator -verbose
	deviceDestination := fmt.Sprintf("platform=%s,name=%s,OS=%s", simulatorPlatform, simulatorDevice, simulatorOsVersion)

	log.LogDetails("* device_destination: %s", deviceDestination)

	derivedDataDir, err := ioutil.TempDir("", "BITRISE-DERIVED-DATA")
	if err != nil {
		log.LogFail("Failed to create derived data path, error: %s\n", err)
	}

	log.LogDetails("* derived_data_dir: %s", derivedDataDir)

	xcodebuildVersion, err := xcodeutil.GetXcodeVersion()
	if err != nil {
		log.LogFail("Failed to get the version of xcodebuild! Error: %s", err)
	}

	log.LogDetails("* xcodebuild_version: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	xcprettyVersion, err := getXcprettyVersion()
	if err != nil {
		log.LogWarn("Failed to get the xcpretty version! Error: %s", err)
	} else {
		log.LogDetails("* xcpretty_version: %s", xcprettyVersion)
	}

	simulator, err := xcodeutil.GetSimulator(simulatorPlatform, simulatorDevice, simulatorOsVersion)
	if err != nil {
		log.LogFail(fmt.Sprintf("failed to get simulator udid, error: %s", err))
	}

	log.LogDetails("* simulator_name: %s, UDID: %s, status: %s", simulator.Name, simulator.SimID, simulator.Status)

	//
	// Start simulator
	if simulator.Status == "Shutdown" {
		log.LogInfo("Booting simulator (%s)...", simulator.SimID)

		if err := xcodeutil.BootSimulator(simulator, xcodebuildVersion); err != nil {
			log.LogFail(fmt.Sprintf("failed to boot simulator, error: %s", err))
		}
	}

	//
	// Run build
	if rawXcodebuildOutput, exitCode, buildErr := runBuild(outputTool, action, projectPath, scheme, cleanBuild, deviceDestination, derivedDataDir); buildErr != nil {
		// --- Outputs
		if err := saveRawOutputToLogFile(rawXcodebuildOutput, false); err != nil {
			log.LogWarn("Failed to save the Raw Output, err: %s", err)
		}
		//
		log.LogWarn("xcode build exit code: %d", exitCode)
		log.LogWarn("xcode build log:\n%s", rawXcodebuildOutput)
		log.LogFail("xcode build failed with error: %s", buildErr)
	}

	//
	// Run test
	rawXcodebuildOutput, exitCode, testErr := runTest(outputTool, action,
		projectPath, scheme, deviceDestination, generateCodeCoverage,
		true, testResultsFilePath, derivedDataDir)

	// --- Outputs
	isRunSuccess := (testErr == nil)

	if err := saveRawOutputToLogFile(rawXcodebuildOutput, isRunSuccess); err != nil {
		log.LogWarn("Failed to save the Raw Output, error %s", err)
	}

	if exportUITestArtifacts {
		if err := saveAttachements(derivedDataDir); err != nil {
			log.LogWarn("Failed to save the screenshots, error %s", err)
		}
	}

	if testErr != nil {
		log.LogWarn("xcode test exit code: %d", exitCode)
		log.LogFail("xcode test failed, error: %s", testErr)
	}

	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded"); err != nil {
		log.LogWarn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}
	//
}