package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
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

func printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, cleanBuild string) {
	log.Println()
	log.Println("========== Configs ==========")
	log.Printf(" * project_path: %s", projectPath)
	log.Printf(" * scheme: %s", scheme)
	log.Printf(" * simulator_device: %s", simulatorDevice)
	log.Printf(" * simulator_os_version: %s", simulatorOsVersion)
	log.Printf(" * is_clean_build: %s", cleanBuild)
	log.Printf(" * project_action: %s", action)
	log.Printf(" * device_destination: %s", deviceDestination)

	cmd := exec.Command("xcodebuild", "-version")
	xcodebuildVersion, err := cmd.CombinedOutput()
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

func isStringFoundInOutput(searchStr, outputToSearchIn string) (bool, error) {
	r, err := regexp.Compile("(?i)" + searchStr)
	if err != nil {
		return false, err
	}
	return r.MatchString(outputToSearchIn), nil
}

func findTestSummaryInOutput(fullOutput string, isRunSucess bool) string {
	splitIdx := -1
	splitDelim := ""
	if !isRunSucess {
		possibleDelimiters := []string{"Testing failed:", "Failing tests:", "** TEST FAILED **"}
		for _, aDelim := range possibleDelimiters {
			splitIdx = strings.LastIndex(fullOutput, aDelim)
			splitDelim = aDelim
			if splitIdx >= 0 {
				break
			}
		}
	} else {
		splitDelim = "** TEST SUCCEEDED **"
		splitIdx = strings.LastIndex(fullOutput, splitDelim)
	}

	if splitIdx < 0 {
		log.Printf(" [!] Could not find the required test result delimiter: %s", splitDelim)
		return ""
	}
	return fullOutput[splitIdx:]
}

func runTest(action, projectPath, scheme, cleanBuild, deviceDestination string, isRetryOnTimeout, isFullOutputMode bool) (string, error) {
	args := []string{action, projectPath, "-scheme", scheme}
	if cleanBuild != "" {
		args = append(args, cleanBuild)
	}
	args = append(args, "test", "-destination", deviceDestination, "-sdk", "iphonesimulator")
	cmd := exec.Command("xcodebuild", args...)

	var outBuffer bytes.Buffer
	var outWriter io.Writer
	if isFullOutputMode {
		outWriter = io.MultiWriter(&outBuffer, os.Stdout)
	} else {
		outWriter = &outBuffer
	}

	cmd.Stdout = outWriter
	cmd.Stderr = outWriter

	fmt.Println()
	log.Println("=> Compiling and running the tests...")
	log.Printf("==> Full command: %#v", cmd)
	if !isFullOutputMode {
		fmt.Println()
		log.Println("=> You selected to only see test results.")
		log.Println("   This can take some time, especially if the code have to be compiled first.")
	}

	err := cmd.Run()
	fullOutputStr := outBuffer.String()
	if err != nil {
		if isTimeoutStrFound, err := isStringFoundInOutput(timeOutMessageIPhoneSimulator, fullOutputStr); err != nil {
			return fullOutputStr, err
		} else if isTimeoutStrFound {
			log.Println("=> Simulator Timeout detected")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(action, projectPath, scheme, cleanBuild, deviceDestination, false, isFullOutputMode)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, err
		}

		if isUITestTimeoutFound, err := isStringFoundInOutput(timeOutMessageUITest, fullOutputStr); err != nil {
			return fullOutputStr, err
		} else if isUITestTimeoutFound {
			log.Println("=> Simulator Timeout detected: isUITestTimeoutFound")
			if isRetryOnTimeout {
				log.Println("==> isRetryOnTimeout=true - retrying...")
				return runTest(action, projectPath, scheme, cleanBuild, deviceDestination, false, isFullOutputMode)
			}
			log.Println(" [!] isRetryOnTimeout=false, no more retry, stopping the test!")
			return fullOutputStr, err
		}

		return fullOutputStr, err
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
	cleanBuild := ""
	if os.Getenv("is_clean_build") == "yes" {
		cleanBuild = "clean"
	}
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
	printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, cleanBuild)

	//
	// Run
	fullOutputStr, runErr := runTest(action, projectPath, scheme, cleanBuild, deviceDestination, true, isFullOutputMode)

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
