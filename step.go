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

const timeOutMessage = "iPhoneSimulator: Timed out waiting"

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
}

func validateRequiredInput(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("[!] Missing required input: %s", key)
	}
	return value, nil
}

func isTimeOutError(outputToSearchIn string) (bool, error) {
	r, err := regexp.Compile("(?i)" + timeOutMessage)
	if err != nil {
		return false, err
	}
	return r.MatchString(outputToSearchIn), nil
}

func runTest(action, projectPath, scheme, cleanBuild, deviceDestination string) error {
	args := []string{action, projectPath, "-scheme", scheme}
	if cleanBuild != "" {
		args = append(args, cleanBuild)
	}
	args = append(args, "test", "-destination", deviceDestination, "-sdk", "iphonesimulator")

	cmd := exec.Command("xcodebuild", args...)

	var outBuffer bytes.Buffer
	outWriter := io.MultiWriter(&outBuffer, os.Stdout)
	cmd.Stdout = outWriter

	errorWriter := io.MultiWriter(&outBuffer, os.Stderr)
	cmd.Stderr = errorWriter

	log.Printf("---- cmd: %#v", cmd)
	if err := cmd.Run(); err != nil {
		if isTimeoutStrFound, err := isTimeOutError(outBuffer.String()); err != nil {
			return err
		} else if isTimeoutStrFound {
			log.Printf("=> Simulator Timeout detected - retrying...")
			return cmd.Run()
		}
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

	//
	// Not required parameters
	cleanBuild := ""
	if os.Getenv("is_clean_build") == "yes" {
		cleanBuild = "clean"
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
	printConfig(projectPath, scheme, simulatorDevice, simulatorOsVersion, action, deviceDestination, cleanBuild)

	//
	// Run
	if err := runTest(action, projectPath, scheme, cleanBuild, deviceDestination); err != nil {
		log.Fatalf("Failed to run xcode test, error: %s", err)
	}
}
