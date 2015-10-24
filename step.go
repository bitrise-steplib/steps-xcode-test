package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// On performance limited OS X hosts (ex: VMs) the iPhone/iOS Simulator might time out
//  while booting. So far it seems that a simple retry solves these issues.

// This boot timeout can happen when running Unit Tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageIPhoneSimulator = "iPhoneSimulator: Timed out waiting"

// This boot timeout can happen when running Xcode (7+) UI tests
//  with Xcode Command Line `xcodebuild`.
const timeOutMessageUITest = "Terminating app due to uncaught exception '_XCTestCaseInterruptionException'"

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
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

func printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination string, cleanBuild bool, generateCodeCoverage bool) {
	log.Println()
	log.Println("========== Configs ==========")
	log.Printf(" * project_path: %s", projectPath)
	log.Printf(" * scheme: %s", scheme)
	log.Printf(" * simulator_device: %s", simulatorDevice)
	log.Printf(" * simulator_os_version: %s", simulatorOsVersion)
	log.Printf(" * is_clean_build: %v", cleanBuild)
	log.Printf(" * project_action: %s", action)
	log.Printf(" * generate_code_coverage_files: %v", generateCodeCoverage)
	log.Printf(" * device_destination: %s", deviceDestination)

	xcodebuildVersion, err := getXcodeVersion()
	if err != nil {
		log.Printf(" [!] Failed to get the version of xcodebuild! Error: %s", err)
	}
	fmt.Println()
	log.Println(" * xcodebuildVersion:")
	fmt.Printf("%s\n", xcodebuildVersion)
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

func runTest(action, projectPath, scheme string, cleanBuild bool, deviceDestination string, generateCodeCoverage bool, isRetryOnTimeout, isFullOutputMode bool) (string, error) {
	args := []string{action, projectPath, "-scheme", scheme}
	if cleanBuild {
		args = append(args, "clean")
	}
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
	cmd := exec.Command("xcodebuild", args...)

	var outBuffer bytes.Buffer
	var outWriter io.Writer
	if isFullOutputMode {
		outWriter = io.MultiWriter(&outBuffer, os.Stdout)
	} else {
		outWriter = &outBuffer
	}

	cmd.Stdin = nil
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter

	fmt.Println()
	log.Println("=> Compiling and running the tests...")
	cmdArgsForPrint := printableCommandArgs(cmd.Args)

	log.Printf("==> Full command: %s", cmdArgsForPrint)
	if !isFullOutputMode {
		fmt.Println()
		log.Println("=> You selected to only see test results.")
		log.Println("   This can take some time, especially if the code have to be compiled first.")
	}

	runErr := cmd.Run()
	fullOutputStr := outBuffer.String()
	if runErr != nil {
		if isStringFoundInOutput(timeOutMessageIPhoneSimulator, fullOutputStr) {
			log.Println("=> Simulator Timeout detected")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(action, projectPath, scheme, false, deviceDestination, generateCodeCoverage, false, isFullOutputMode)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, runErr
		}

		if isStringFoundInOutput(timeOutMessageUITest, fullOutputStr) {
			log.Println("=> Simulator Timeout detected: isUITestTimeoutFound")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(action, projectPath, scheme, false, deviceDestination, generateCodeCoverage, false, isFullOutputMode)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, runErr
		}

		return fullOutputStr, runErr
	}

	return fullOutputStr, nil
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

	//
	// Not required parameters
	cleanBuild := (os.Getenv("is_clean_build") == "yes")
	generateCodeCoverage := (os.Getenv("generate_code_coverage_files") == "yes")
	isFullOutputMode := !(os.Getenv("is_full_output") == "no")

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
	printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, cleanBuild, generateCodeCoverage)

	//
	// Run
	fullOutputStr, runErr := runTest(action, projectPath, scheme, cleanBuild, deviceDestination, generateCodeCoverage, true, isFullOutputMode)

	//
	isRunSuccess := (runErr == nil)
	if isRunSuccess {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "succeeded")
	} else {
		exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed")
	}
	testResultsSummary := findTestSummaryInOutput(fullOutputStr, isRunSuccess)
	if testResultsSummary == "" {
		testResultsSummary = fmt.Sprintf(" [!] No test summary found in the output - most likely it was a compilation error.\n\n Full output was: %s", fullOutputStr)
	}
	exportEnvironmentWithEnvman("BITRISE_XCODE_TEST_FULL_RESULTS_TEXT", testResultsSummary)

	if !isFullOutputMode {
		fmt.Println()
		fmt.Println("========= TEST RESULTS: =========")
		fmt.Println(testResultsSummary)
		fmt.Println()
	}

	if runErr != nil {
		log.Fatalf("xcode test failed with error: %s", runErr)
	}
}
