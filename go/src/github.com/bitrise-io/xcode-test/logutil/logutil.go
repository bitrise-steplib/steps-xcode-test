package logutil

import (
	"fmt"
	"os"

	cmd "github.com/bitrise-io/xcode-test/command"
)

// LogFail ...
func LogFail(format string, v ...interface{}) {
	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
		LogWarn("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}

	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[31;1m%s\x1b[0m\n", errorMsg)
	os.Exit(1)
}

// LogWarn ...
func LogWarn(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[33;1m%s\x1b[0m\n", errorMsg)
}

// LogInfo ...
func LogInfo(format string, v ...interface{}) {
	fmt.Println()
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[34;1m%s\x1b[0m\n", errorMsg)
}

// LogDetails ...
func LogDetails(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("  %s\n", errorMsg)
}

// LogDone ...
func LogDone(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("  \x1b[32;1m%s\x1b[0m\n", errorMsg)
}

// LogConfigs ...
func LogConfigs(
	projectPath,
	scheme,
	simulatorPlatform,
	simulatorDevice,
	simulatorOsVersion,
	testResultsFilePath,
	isCleanBuild,
	generateCodeCoverageFiles,
	outputTool,
	exportUITestArtifactsStr,
	testOptions,
	singleBuild,
	shouldBuildBeforeTest string) {
	LogInfo("Configs:")
	LogDetails("* project_path: %s", projectPath)
	LogDetails("* scheme: %s", scheme)
	LogDetails("* is_clean_build: %v", isCleanBuild)
	LogDetails("* xcodebuild_test_options: %s", testOptions)
	LogDetails("* single_build: %s", singleBuild)
	LogDetails("* should_build_before_test: %s", shouldBuildBeforeTest)
	fmt.Println()
	LogDetails("* simulator_platform: %s", simulatorPlatform)
	LogDetails("* simulator_device: %s", simulatorDevice)
	LogDetails("* simulator_os_version: %s", simulatorOsVersion)
	fmt.Println()
	LogDetails("* output_tool: %s", outputTool)
	LogDetails("* test_result_file_path: %s", testResultsFilePath)
	LogDetails("* generate_code_coverage_files: %v", generateCodeCoverageFiles)
	LogDetails("* export_uitest_artifacts: %v", exportUITestArtifactsStr)
}
