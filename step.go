package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.

// This boot timeout can happen when running Unit Tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageIPhoneSimulator = "iPhoneSimulator: Timed out waiting"

// This boot timeout can happen when running Xcode (7+) UI tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageUITest = "Terminating app due to uncaught exception '_XCTestCaseInterruptionException'"

func printFatal(exitCode int, format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	log.Printf("\x1b[31;1m%s\x1b[0m", errorMsg)
	os.Exit(exitCode)
}

func printError(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	log.Printf("\x1b[31;1m%s\x1b[0m", errorMsg)
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	fmt.Println()
	log.Printf("Exporting: %s", keyStr)
	envman := exec.Command("envman", "add", "--key", keyStr)
	envman.Stdin = strings.NewReader(valueStr)
	envman.Stdout = os.Stdout
	envman.Stderr = os.Stderr
	return envman.Run()
}

func getXcodeVersion() (string, error) {
	cmd := exec.Command("xcodebuild", "-version")
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xcodebuild -version failed, err: %s, details: %s", err, string(outBytes))
	}

	return string(outBytes), nil
}

func getXcprettyVersion() (string, error) {
	cmd := exec.Command("xcpretty", "-version")
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xcpretty -version failed, err: %s, details: %s", err, string(outBytes))
	}

	return string(outBytes), nil
}

// Printlnf ...
func Printlnf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	fmt.Println()
}

func printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, outputTool string, cleanBuild bool, generateCodeCoverage bool) {
	fmt.Println()
	fmt.Println("========== Configs ==========")
	Printlnf(" * project_path: %s", projectPath)
	Printlnf(" * scheme: %s", scheme)
	Printlnf(" * simulator_device: %s", simulatorDevice)
	Printlnf(" * simulator_os_version: %s", simulatorOsVersion)
	Printlnf(" * is_clean_build: %v", cleanBuild)
	Printlnf(" * project_action: %s", action)
	Printlnf(" * generate_code_coverage_files: %v", generateCodeCoverage)
	Printlnf(" * device_destination: %s", deviceDestination)
	Printlnf(" * output_tool: %s", outputTool)

	if outputTool == "xcpretty" {
		version, err := getXcprettyVersion()
		if err != nil || version == "" {
			log.Fatal(`
 (!) xcpretty is not installed
		 For xcpretty installation see: 'https://github.com/supermarin/xcpretty',
		 or use 'xcodebuild' as 'output_tool'.`)
		}
		Printlnf(" * xcpretty version: %s", strings.TrimSpace(version))
	}

	xcodebuildVersion, err := getXcodeVersion()
	if err != nil {
		log.Printf(" [!] Failed to get the version of xcodebuild! Error: %s", err)
	}
	Printlnf(" * xcodebuild version: %s", strings.TrimSpace(xcodebuildVersion))
	fmt.Println("=============================")
	fmt.Println()
}

func validateRequiredInput(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("[!] Missing required input: %s", key)
	}
	return value, nil
}

func isStringFoundInOutput(searchStr, outputToSearchIn string) bool {
	r, err := regexp.Compile("(?i)" + searchStr)
	if err != nil {
		log.Printf(" [!] Failed to compile regexp: %s", err)
		return false
	}
	return r.MatchString(outputToSearchIn)
}

func printableCommandArgs(fullCommandArgs []string) string {
	cmdArgsDecorated := []string{}
	for idx, anArg := range fullCommandArgs {
		quotedArg := strconv.Quote(anArg)
		if idx == 0 {
			quotedArg = anArg
		}
		cmdArgsDecorated = append(cmdArgsDecorated, quotedArg)
	}

	return strings.Join(cmdArgsDecorated, " ")
}

// createXcodebuildCmd ...
func createXcodebuildCmd(xcodebuildArgs ...string) *exec.Cmd {
	return exec.Command("xcodebuild", xcodebuildArgs...)
}

// createXcprettyCmd ...
func createXcprettyCmd(testResultsFilePath string) *exec.Cmd {
	prettyArgs := []string{"--color"}
	if testResultsFilePath != "" {
		prettyArgs = append(prettyArgs, "--report", "html", "--output", testResultsFilePath)
	}
	return exec.Command("xcpretty", prettyArgs...)
}

// CreateBufferedWriter ...
func CreateBufferedWriter(buff *bytes.Buffer, writers ...io.Writer) io.Writer {
	if len(writers) > 0 {
		allWriters := append([]io.Writer{buff}, writers...)
		return io.MultiWriter(allWriters...)
	}
	return io.Writer(buff)
}

// runXcodeBuildCmd ...
func runXcodeBuildCmd(useStdOut bool, args ...string) (string, int, error) {
	// command
	buildCmd := createXcodebuildCmd(args...)
	// output buffer
	var outBuffer bytes.Buffer
	// additional output writers, like StdOut
	outWritters := []io.Writer{}
	if useStdOut {
		outWritters = append(outWritters, os.Stdout)
	}
	// unify as a single writer
	outWritter := CreateBufferedWriter(&outBuffer, outWritters...)
	// and set the writer
	buildCmd.Stdin = nil
	buildCmd.Stdout = outWritter
	buildCmd.Stderr = outWritter

	cmdArgsForPrint := printableCommandArgs(buildCmd.Args)
	log.Printf("==> Full command: $ %s", cmdArgsForPrint)

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
	buildCmd := createXcodebuildCmd(args...)
	prettyCmd := createXcprettyCmd(testResultsFilePath)
	//
	var buildOutBuffer bytes.Buffer
	//
	pipeReader, pipeWriter := io.Pipe()
	//
	// build outputs:
	// - write it into a buffer
	// - write it into the pipe, which will be fed into xcpretty
	buildOutWriters := []io.Writer{pipeWriter}
	buildOutWriter := CreateBufferedWriter(&buildOutBuffer, buildOutWriters...)
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

	log.Printf("==> Full command: $ set -o pipefail && %s | %v",
		printableCommandArgs(buildCmd.Args),
		printableCommandArgs(prettyCmd.Args))

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
	deviceDestination string) (string, int, error) {

	args := []string{action, projectPath, "-scheme", scheme}
	if cleanBuild {
		args = append(args, "clean")
	}
	args = append(args, "build", "-destination", deviceDestination)

	log.Println("=> Building the project...")

	if outputTool == "xcpretty" {
		return runPrettyXcodeBuildCmd(false, "", args...)
	}
	return runXcodeBuildCmd(false, args...)
}

func runTest(outputTool, action, projectPath, scheme string,
	deviceDestination string, generateCodeCoverage bool,
	isRetryOnTimeout bool,
	testResultsFilePath string) (string, int, error) {

	handleTestError := func(fullOutputStr string, exitCode int, testError error) (string, int, error) {
		// fmt.Printf("\n\nfullOutputStr:\n\n%s", fullOutputStr)
		if isStringFoundInOutput(timeOutMessageIPhoneSimulator, fullOutputStr) {
			log.Println("=> Simulator Timeout detected")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(outputTool, action,
					projectPath, scheme, deviceDestination, generateCodeCoverage,
					false, testResultsFilePath)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, exitCode, testError
		}

		if isStringFoundInOutput(timeOutMessageUITest, fullOutputStr) {
			log.Println("=> Simulator Timeout detected: isUITestTimeoutFound")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(outputTool, action,
					projectPath, scheme, deviceDestination, generateCodeCoverage,
					false, testResultsFilePath)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
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
	args = append(args, "build", "test", "-destination", deviceDestination)

	if generateCodeCoverage {
		args = append(args, "GCC_INSTRUMENT_PROGRAM_FLOW_ARCS=YES")
		args = append(args, "GCC_GENERATE_TEST_COVERAGE_FILES=YES")
	}

	fmt.Println()
	log.Println("=> Running the tests...")

	var out string
	var exit int
	var err error
	if outputTool == "xcpretty" {
		out, exit, err = runPrettyXcodeBuildCmd(true, testResultsFilePath, args...)
	} else {
		out, exit, err = runXcodeBuildCmd(true, args...)
	}

	if err != nil {
		return handleTestError(out, exit, err)
	}
	return out, exit, nil
}

func main() {
	//
	// Required parameters
	projectPath, err := validateRequiredInput("project_path")
	if err != nil {
		log.Fatalf("Input validation failed, err: %s", err)
	}

	scheme, err := validateRequiredInput("scheme")
	if err != nil {
		log.Fatalf("Input validation failed, err: %s", err)
	}

	simulatorDevice, err := validateRequiredInput("simulator_device")
	if err != nil {
		log.Fatalf("Input validation failed, err: %s", err)
	}

	simulatorOsVersion, err := validateRequiredInput("simulator_os_version")
	if err != nil {
		log.Fatalf("Input validation failed, err: %s", err)
	}
	testResultsFilePath, err := validateRequiredInput("test_results_file_path")
	if err != nil {
		log.Fatalf("Input validation failed, err: %s", err)
	}

	//
	// Not required parameters
	cleanBuild := (os.Getenv("is_clean_build") == "yes")
	generateCodeCoverage := (os.Getenv("generate_code_coverage_files") == "yes")
	outputTool := os.Getenv("output_tool")
	if outputTool != "xcpretty" && outputTool != "xcodebuild" {
		log.Fatalf("Invalid output_tool: (%s), valid options: xcpretty, xcodebuild", outputTool)
	}

	//
	// Project-or-Workspace flag
	action := ""
	if strings.HasSuffix(projectPath, ".xcodeproj") {
		action = "-project"
	} else if strings.HasSuffix(projectPath, ".xcworkspace") {
		action = "-workspace"
	} else {
		log.Fatalf("Failed to get valid project file (invalid project file): %s", projectPath)
	}

	//
	// Device Destination
	// xcodebuild -project ./BitriseSampleWithYML.xcodeproj -scheme BitriseSampleWithYML  test -destination "platform=iOS Simulator,name=iPhone 6 Plus,OS=latest" -sdk iphonesimulator -verbose
	deviceDestination := fmt.Sprintf("platform=iOS Simulator,name=%s,OS=%s", simulatorDevice, simulatorOsVersion)

	//
	// Print configs
	printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, outputTool, cleanBuild, generateCodeCoverage)

	//
	// Run build
	if buildOutputStr, exitCode, buildErr := runBuild(outputTool, action, projectPath, scheme, cleanBuild, deviceDestination); buildErr != nil {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed")

		if buildErr != nil {
			log.Printf("xcode build log:\n%s", buildOutputStr)
			printFatal(exitCode, "xcode build failed with error: %s\n", buildErr)
		}
	}

	//
	// Run test
	_, exitCode, testErr := runTest(outputTool, action,
		projectPath, scheme, deviceDestination, generateCodeCoverage,
		true, testResultsFilePath)

	isRunSuccess := (testErr == nil)
	if isRunSuccess {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded")
	} else {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed")
	}

	if testErr != nil {
		printFatal(exitCode, "xcode test failed with error: %s", testErr)
	}
}
