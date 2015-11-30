package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
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

func printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, outputTool string, cleanBuild bool, generateCodeCoverage bool) {
	fmt.Println()
	log.Println("========== Configs ==========")
	log.Printf(" * project_path: %s", projectPath)
	log.Printf(" * scheme: %s", scheme)
	log.Printf(" * simulator_device: %s", simulatorDevice)
	log.Printf(" * simulator_os_version: %s", simulatorOsVersion)
	log.Printf(" * is_clean_build: %v", cleanBuild)
	log.Printf(" * project_action: %s", action)
	log.Printf(" * generate_code_coverage_files: %v", generateCodeCoverage)
	log.Printf(" * device_destination: %s", deviceDestination)
	log.Printf(" * output_tool: %s", outputTool)

	xcodebuildVersion, err := getXcodeVersion()
	if err != nil {
		log.Printf(" [!] Failed to get the version of xcodebuild! Error: %s", err)
	}
	fmt.Println()
	log.Println(" * xcodebuildVersion:")
	fmt.Printf("%s\n", xcodebuildVersion)
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

func findFirstDelimiter(searchIn string, searchForDelimiters []string) (foundIdx int, foundDelim string) {
	foundIdx = -1
	for _, aDelim := range searchForDelimiters {
		aDelimFoundIdx := strings.Index(searchIn, aDelim)
		if aDelimFoundIdx >= 0 {
			if foundIdx == -1 || aDelimFoundIdx < foundIdx {
				foundIdx = aDelimFoundIdx
				foundDelim = aDelim
			}
		}
	}
	return foundIdx, foundDelim
}

func findTestSummaryInOutput(fullOutput string, isRunSucess bool) string {
	// using a list of possible delimiters, because the actual order
	//  of these delimiters varies in Xcode CLT's output,
	//  so we'll try to find the first occurance of any of the listed
	//  delimiters
	possibleDelimiters := []string{}
	if !isRunSucess {
		// Failed
		possibleDelimiters = []string{"Testing failed:", "Failing tests:", "** TEST FAILED **"}
	} else {
		// Success
		possibleDelimiters = []string{"Test Suite ", "** TEST SUCCEEDED **"}
	}

	splitIdx, _ := findFirstDelimiter(fullOutput, possibleDelimiters)
	if splitIdx < 0 {
		log.Println(" [!] Could not find any of the required test result delimiters")
		return ""
	}
	return fullOutput[splitIdx:]
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

func runXcodeBuildCmd(useStdOut bool, args ...string) (string, int, error) {
	buildCmd := exec.Command("xcodebuild", args...)

	cmdArgsForPrint := printableCommandArgs(buildCmd.Args)
	log.Printf("==> Full command: %s", cmdArgsForPrint)

	var outBuffer bytes.Buffer
	outWritter := io.Writer(&outBuffer)
	if useStdOut {
		outWritter = io.MultiWriter(&outBuffer, os.Stdout)
	}

	buildCmd.Stdin = nil
	buildCmd.Stdout = outWritter
	buildCmd.Stderr = outWritter

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

func runPrettyXcodeBuildCmd(useStdOut, reportHTML bool, args ...string) (string, int, error) {
	buildCmd := exec.Command("xcodebuild", args...)

	cmdArgsForPrint := printableCommandArgs(buildCmd.Args)
	log.Printf("==> Full command: %s", cmdArgsForPrint)

	prettyArgs := []string{}
	if reportHTML {
		deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
		reportPath := path.Join(deployDir, "BitriseXcodeTestReport.html")
		log.Printf("report path: %s", reportPath)
		prettyArgs = append(prettyArgs, "--report", "html", "--output", reportPath)
	}
	prettyCmd := exec.Command("xcpretty", prettyArgs...)

	var prettyOutBuffer bytes.Buffer
	prettyOutWritter := io.Writer(&prettyOutBuffer)
	if useStdOut {
		prettyOutWritter = io.MultiWriter(&prettyOutBuffer, os.Stdout)
	}

	reader, writer := io.Pipe()
	var buildOutBuffer bytes.Buffer
	buildOutWriter := io.MultiWriter(writer, &buildOutBuffer)

	buildCmd.Stdin = nil
	buildCmd.Stdout = buildOutWriter
	buildCmd.Stderr = buildOutWriter

	prettyCmd.Stdin = reader
	prettyCmd.Stdout = prettyOutWritter
	prettyCmd.Stderr = prettyOutWritter

	if err := buildCmd.Start(); err != nil {
		return buildOutBuffer.String(), 1, err
	}
	if err := prettyCmd.Start(); err != nil {
		return prettyOutBuffer.String(), 1, err
	}

	if err := buildCmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok {
				return prettyOutBuffer.String(), 1, errors.New("Failed to cast exit status")
			}
			return prettyOutBuffer.String(), waitStatus.ExitStatus(), err
		}
		return prettyOutBuffer.String(), 1, err
	}
	if err := writer.Close(); err != nil {
		return prettyOutBuffer.String(), 1, err
	}

	if err := prettyCmd.Wait(); err != nil {
		return prettyOutBuffer.String(), 1, err
	}

	return prettyOutBuffer.String(), 0, nil
}

func runBuild(outputTool, action, projectPath, scheme string, cleanBuild bool, deviceDestination string) (string, int, error) {
	args := []string{action, projectPath, "-scheme", scheme}
	if cleanBuild {
		args = append(args, "clean")
	}
	args = append(args, "build", "-destination", deviceDestination)

	log.Println("=> Building the project...")

	if outputTool == "xcpretty" {
		return runPrettyXcodeBuildCmd(false, false, args...)
	}
	return runXcodeBuildCmd(false, args...)
}

func runTest(outputTool, action, projectPath, scheme string, deviceDestination string, generateCodeCoverage bool, isRetryOnTimeout bool) (string, int, error) {
	handleTestError := func(fullOutputStr string, exitCode int, testError error) (string, int, error) {
		if isStringFoundInOutput(timeOutMessageIPhoneSimulator, fullOutputStr) {
			log.Println("=> Simulator Timeout detected")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(outputTool, action, projectPath, scheme, deviceDestination, generateCodeCoverage, false)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, exitCode, testError
		}

		if isStringFoundInOutput(timeOutMessageUITest, fullOutputStr) {
			log.Println("=> Simulator Timeout detected: isUITestTimeoutFound")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(outputTool, action, projectPath, scheme, deviceDestination, generateCodeCoverage, false)
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
		out, exit, err = runPrettyXcodeBuildCmd(true, true, args...)
	} else {
		out, exit, err = runXcodeBuildCmd(true, args...)
	}

	if err != nil {
		return handleTestError(out, exit, err)
	}
	return out, exit, nil
}

// WriteBytesToFileWithPermission ...
//  copied from the Bitrise go-utils
//  repository: https://github.com/bitrise-io/go-utils/blob/master/fileutil/fileutil.go
func WriteBytesToFileWithPermission(pth string, fileCont []byte, perm os.FileMode) error {
	if pth == "" {
		return errors.New("No path provided")
	}

	var file *os.File
	var err error
	if perm == 0 {
		file, err = os.Create(pth)
	} else {
		// same as os.Create, but with a specified permission
		//  the flags are copy-pasted from the official
		//  os.Create func: https://golang.org/src/os/file.go?s=7327:7366#L244
		file, err = os.OpenFile(pth, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	}
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Println(" [!] Failed to close file:", err)
		}
	}()

	if _, err := file.Write(fileCont); err != nil {
		return err
	}

	return nil
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
	testSummaryFilePath, err := validateRequiredInput("test_results_file_path")
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
	if outputTool == "xcpretty" {
		version, err := getXcprettyVersion()
		if err != nil {
			log.Fatalf("Failed to check xcpretty version, err: %s", err)
		}
		if version == "" {
			log.Fatalf("xcpertty is not installed")
		}
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

		fmt.Println()
		log.Printf("=> Saving build summary into file: %s", testSummaryFilePath)
		if err := WriteBytesToFileWithPermission(testSummaryFilePath, []byte(buildOutputStr), 0); err != nil {
			printError(" [!] Failed to write summary into file, error: %s", err)
		}

		if buildErr != nil {
			log.Printf("xcode build log:\n%s", buildOutputStr)
			printFatal(exitCode, "xcode build failed with error: %s\n", buildErr)
		}
	}

	//
	// Run test
	testOutputStr, exitCode, testErr := runTest(outputTool, action, projectPath, scheme, deviceDestination, generateCodeCoverage, true)

	isRunSuccess := (testErr == nil)
	if isRunSuccess {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded")
	} else {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed")
	}

	fmt.Println()
	log.Printf("=> Saving test summary into file: %s", testSummaryFilePath)
	if err := WriteBytesToFileWithPermission(testSummaryFilePath, []byte(testOutputStr), 0); err != nil {
		printError(" [!] Failed to write summary into file, error: %s", err)
	}

	if testErr != nil {
		printFatal(exitCode, "xcode test failed with error: %s", testErr)
	}
}
