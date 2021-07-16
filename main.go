package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	bitriseConfigs "github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/go-steputils/output"
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/ziputil"
	simulator "github.com/bitrise-io/go-xcode/simulator"
	"github.com/bitrise-io/go-xcode/utility"
	cache "github.com/bitrise-io/go-xcode/xcodecache"
	cmd "github.com/bitrise-steplib/steps-xcode-test/command"
	"github.com/bitrise-steplib/steps-xcode-test/models"
)

const (
	minSupportedXcodeMajorVersion = 6
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
	testRunnerNeverBeganExecuting            = `Test runner never began executing tests after launching.`
)

var testRunnerErrorPatterns = []string{
	timeOutMessageIPhoneSimulator,
	timeOutMessageUITest,
	earlyUnexpectedExit,
	failureAttemptingToLaunch,
	failedToBackgroundTestRunner,
	appStateIsStillNotRunning,
	appAccessibilityIsNotLoaded,
	testRunnerFailedToInitializeForUITesting,
	timedOutRegisteringForTestingEvent,
	testRunnerNeverBeganExecuting,
}

const simulatorShutdownState = "Shutdown"

const (
	xcodebuildTool = "xcodebuild"
	xcprettyTool   = "xcpretty"
)

var xcodeCommandEnvs = []string{"NSUnbufferedIO=YES"}

// Step ...
type Step struct{}

// NewStep ...
func NewStep() Step {
	return Step{}
}

// Input ...
type Input struct {
	// Project Parameters
	ProjectPath string `env:"project_path,required"`
	Scheme      string `env:"scheme,required"`
	TestPlan    string `env:"test_plan"`

	// Simulator Configs
	SimulatorPlatform  string `env:"simulator_platform,required"`
	SimulatorDevice    string `env:"simulator_device,required"`
	SimulatorOsVersion string `env:"simulator_os_version,required"`

	// Test Repetition
	TestRepetitionMode             string `env:"test_repetition_mode,opt[none,until_failure,retry_on_failure,up_until_maximum_repetitions]"`
	MaximumTestRepetitions         int    `env:"maximum_test_repetitions,required"`
	RelaunchTestsForEachRepetition bool   `env:"relaunch_tests_for_each_repetition,opt[yes,no]"`

	// Test Run Configs
	OutputTool            string `env:"output_tool,opt[xcpretty,xcodebuild]"`
	IsCleanBuild          bool   `env:"is_clean_build,opt[yes,no]"`
	IsSingleBuild         bool   `env:"single_build,opt[true,false]"`
	ShouldBuildBeforeTest bool   `env:"should_build_before_test,opt[yes,no]"`

	RetryTestsOnFailure       bool `env:"should_retry_test_on_fail,opt[yes,no]"`
	DisableIndexWhileBuilding bool `env:"disable_index_while_building,opt[yes,no]"`
	GenerateCodeCoverageFiles bool `env:"generate_code_coverage_files,opt[yes,no]"`
	HeadlessMode              bool `env:"headless_mode,opt[yes,no]"`

	TestOptions         string `env:"xcodebuild_test_options"`
	XcprettyTestOptions string `env:"xcpretty_test_options"`

	// Debug
	Verbose                     bool   `env:"verbose,opt[yes,no]"`
	CollectSimulatorDiagnostics string `env:"collect_simulator_diagnostics,opt[always,on_failure,never]"`

	// Output export
	DeployDir             string `env:"BITRISE_DEPLOY_DIR"`
	ExportUITestArtifacts bool   `env:"export_uitest_artifacts,opt[true,false]"`

	CacheLevel string `env:"cache_level,opt[none,swift_packages]"`
}

// Config ...
type Config struct {
	ProjectPath string
	Scheme      string
	TestPlan    string

	XcodeMajorVersion int
	SimulatorID       string
	IsSimulatorBooted bool

	TestRepetitionMode            string
	MaximumTestRepetitions        int
	RelaunchTestForEachRepetition bool

	OutputTool         string
	IsCleanBuild       bool
	IsSingleBuild      bool
	BuildBeforeTesting bool

	RetryTestsOnFailure       bool
	DisableIndexWhileBuilding bool
	GenerateCodeCoverageFiles bool
	HeadlessMode              bool

	XcodebuildTestOptions string
	XcprettyOptions       string

	Verbose        bool
	SimulatorDebug exportCondition

	DeployDir             string
	ExportUITestArtifacts bool

	CacheLevel string
}

// ProcessConfig ...
func (s Step) ProcessConfig() (Config, error) {
	var input Input
	if err := stepconf.Parse(&input); err != nil {
		return Config{}, fmt.Errorf("issue with input: %s", err)
	}

	stepconf.Print(input)
	fmt.Println()

	// validate Xcode version
	xcodebuildVersion, err := utility.GetXcodeVersion()
	if err != nil {
		return Config{}, fmt.Errorf("failed to determine Xcode version, error: %s", err)
	}
	log.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	xcodeMajorVersion := xcodebuildVersion.MajorVersion
	if xcodeMajorVersion < minSupportedXcodeMajorVersion {
		return Config{}, fmt.Errorf("invalid Xcode major version (%d), should not be less then min supported: %d", xcodeMajorVersion, minSupportedXcodeMajorVersion)
	}

	if xcodeMajorVersion < 11 && input.TestPlan != "" {
		return Config{}, fmt.Errorf("input Test Plan incompatible with Xcode %d, at least Xcode 11 required", xcodeMajorVersion)
	}

	// validate headless mode
	headlessMode := input.HeadlessMode
	if xcodeMajorVersion < 9 && input.HeadlessMode {
		log.Warnf("Headless mode is enabled but it's only available with Xcode 9.x or newer.")
		headlessMode = false
	}

	// validate export UITest artifacts
	exportUITestArtifacts := input.ExportUITestArtifacts
	if input.ExportUITestArtifacts && xcodeMajorVersion >= 11 {
		// The test result bundle (xcresult) structure changed in Xcode 11:
		// it does not contains TestSummaries.plist nor Attachments directly.
		log.Warnf("Export UITest Artifacts (export_uitest_artifacts) turned on, but Xcode version >= 11. The test result bundle structure changed in Xcode 11 it does not contain TestSummaries.plist and Attachments directly, nothing to export.")
		exportUITestArtifacts = false
	}

	// validate simulator diagnosis mode
	simulatorDebug := parseExportCondition(input.CollectSimulatorDiagnostics)
	if simulatorDebug == invalid {
		return Config{}, fmt.Errorf("internal error, unexpected value (%s) for collect_simulator_diagnostics", input.CollectSimulatorDiagnostics)
	}
	if simulatorDebug != never && xcodeMajorVersion < 10 {
		log.Warnf("Collecting Simulator diagnostics is not available below Xcode version 10, current Xcode version: %s", xcodeMajorVersion)
		simulatorDebug = never
	}

	// validate project path
	projectPath, err := pathutil.AbsPath(input.ProjectPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute project path, error: %s", err)
	}
	if filepath.Ext(projectPath) != ".xcodeproj" && filepath.Ext(projectPath) != ".xcworkspace" {
		return Config{}, fmt.Errorf("invalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", projectPath)
	}

	// validate simulator related inputs
	var sim simulator.InfoModel
	var osVersion string

	platform := strings.TrimSuffix(input.SimulatorPlatform, " Simulator")
	// Retry gathering device information since xcrun simctl list can fail to show the complete device list
	if err = retry.Times(3).Wait(10 * time.Second).Try(func(attempt uint) error {
		var errGetSimulator error
		if input.SimulatorOsVersion == "latest" {
			var simulatorDevice = input.SimulatorDevice
			if simulatorDevice == "iPad" {
				log.Warnf("Given device (%s) is deprecated, using iPad Air (3rd generation)...", simulatorDevice)
				simulatorDevice = "iPad Air (3rd generation)"
			}

			sim, osVersion, errGetSimulator = simulator.GetLatestSimulatorInfoAndVersion(platform, simulatorDevice)
		} else {
			normalizedOsVersion := input.SimulatorOsVersion
			osVersionSplit := strings.Split(normalizedOsVersion, ".")
			if len(osVersionSplit) > 2 {
				normalizedOsVersion = strings.Join(osVersionSplit[0:2], ".")
			}
			osVersion = fmt.Sprintf("%s %s", platform, normalizedOsVersion)

			sim, errGetSimulator = simulator.GetSimulatorInfo(osVersion, input.SimulatorDevice)
		}

		if errGetSimulator != nil {
			log.Warnf("attempt %d to get simulator udid failed with error: %s", attempt, errGetSimulator)
		}

		return errGetSimulator
	}); err != nil {
		return Config{}, fmt.Errorf("simulator UDID lookup failed: %s", err)
	}

	log.Infof("Simulator infos")
	log.Printf("* simulator_name: %s, version: %s, UDID: %s, status: %s", sim.Name, osVersion, sim.ID, sim.Status)

	// Device Destination
	deviceDestination := fmt.Sprintf("id=%s", sim.ID)

	log.Printf("* device_destination: %s", deviceDestination)
	fmt.Println()

	if input.TestRepetitionMode != none && xcodeMajorVersion < 13 {
		return Config{}, errors.New("Test Repetition Mode (test_repetition_mode) is not available below Xcode 13")
	}

	if input.TestRepetitionMode != none && input.MaximumTestRepetitions < 2 {
		return Config{}, fmt.Errorf("invalid number of Maximum Test Repetitions (maximum_test_repetitions): %d, should be more than 1", input.MaximumTestRepetitions)
	}

	if input.RelaunchTestsForEachRepetition && input.TestRepetitionMode == none {
		return Config{}, errors.New("Relaunch Tests for Each Repetition (relaunch_tests_for_each_repetition) cannot be used if Test Repetition Mode (test_repetition_mode) is 'none'")
	}

	if input.RetryTestsOnFailure && xcodeMajorVersion > 12 {
		return Config{}, errors.New("Should retry test on failure? (should_retry_test_on_fail) is not available above Xcode 12; use test_repetition_mode=retry_on_failure instead")
	}

	return Config{
		ProjectPath: projectPath,
		Scheme:      input.Scheme,
		TestPlan:    input.TestPlan,

		XcodeMajorVersion: int(xcodeMajorVersion),
		SimulatorID:       sim.ID,
		IsSimulatorBooted: sim.Status != simulatorShutdownState,

		TestRepetitionMode:            input.TestRepetitionMode,
		MaximumTestRepetitions:        input.MaximumTestRepetitions,
		RelaunchTestForEachRepetition: input.RelaunchTestsForEachRepetition,

		OutputTool:         input.OutputTool,
		IsCleanBuild:       input.IsCleanBuild,
		IsSingleBuild:      input.IsSingleBuild,
		BuildBeforeTesting: input.ShouldBuildBeforeTest,

		RetryTestsOnFailure:       input.RetryTestsOnFailure,
		DisableIndexWhileBuilding: input.DisableIndexWhileBuilding,
		GenerateCodeCoverageFiles: input.GenerateCodeCoverageFiles,
		HeadlessMode:              headlessMode,

		XcodebuildTestOptions: input.TestOptions,
		XcprettyOptions:       input.XcprettyTestOptions,

		Verbose:        input.Verbose,
		SimulatorDebug: simulatorDebug,

		DeployDir:             input.DeployDir,
		ExportUITestArtifacts: exportUITestArtifacts,

		CacheLevel: input.CacheLevel,
	}, nil
}

// Result ...
type Result struct {
	XcresultPath             string
	XcodebuildBuildLog       string
	XcodebuildTestLog        string
	SimulatorDiagnosticsPath string
}

// InstallDeps ...
func (s Step) InstallDeps(xcpretty bool) error {
	if !xcpretty {
		return nil
	}

	xcprettyVersion, err := InstallXcpretty()
	if err != nil {
		_, err = handleXcprettyInstallError(err)
		if err != nil {
			return fmt.Errorf("an error occured during installing xcpretty: %s", err)
		}
	} else {
		log.Printf("- xcprettyVersion: %s", xcprettyVersion.String())
		fmt.Println()
	}
	return nil
}

// Run ...
func (s Step) Run(cfg Config) (Result, error) {
	log.SetEnableDebugLog(cfg.Verbose)

	// Boot simulator
	if cfg.SimulatorDebug != never {
		log.Infof("Enabling Simulator verbose log for better diagnostics")
		// Boot the simulator now, so verbose logging can be enabled and it is kept booted after running tests,
		// this helps to collect more detailed debug info
		if err := simulatorBoot(cfg.SimulatorID); err != nil {
			return Result{}, fmt.Errorf("%v", err)
		}
		if err := simulatorEnableVerboseLog(cfg.SimulatorID); err != nil {
			return Result{}, fmt.Errorf("%v", err)
		}

		fmt.Println()
	}

	if !cfg.IsSimulatorBooted && !cfg.HeadlessMode {
		log.Infof("Booting simulator (%s)...", cfg.SimulatorID)

		if err := simulator.BootSimulator(cfg.SimulatorID, cfg.XcodeMajorVersion); err != nil {
			if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
				log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			}
			return Result{}, fmt.Errorf("failed to boot simulator, error: %s", err)
		}

		progress.NewDefaultWrapper("Waiting for simulator boot").WrapAction(func() {
			time.Sleep(60 * time.Second)
		})

		fmt.Println()
	}

	// Run build
	result := Result{}

	projectFlag := "-project"
	if filepath.Ext(cfg.ProjectPath) == ".xcworkspace" {
		projectFlag = "-workspace"
	}

	buildParams := models.XcodebuildParams{
		Action:                    projectFlag,
		ProjectPath:               cfg.ProjectPath,
		Scheme:                    cfg.Scheme,
		DeviceDestination:         fmt.Sprintf("id=%s", cfg.SimulatorID),
		CleanBuild:                cfg.IsCleanBuild,
		DisableIndexWhileBuilding: cfg.DisableIndexWhileBuilding,
	}

	if !cfg.IsSingleBuild {
		buildLog, exitCode, err := runBuild(buildParams, cfg.OutputTool)
		result.XcodebuildBuildLog = buildLog
		if err != nil {
			log.Warnf("xcode build exit code: %d", exitCode)
			log.Warnf("xcode build log:\n%s", buildLog)
			log.Errorf("xcode build failed with error: %s", err)
			return result, err
		}
	}

	// Run test
	tempDir, err := ioutil.TempDir("", "XCUITestOutput")
	if err != nil {
		return result, fmt.Errorf("could not create test output temporary directory: %s", err)
	}
	xcresultPath := path.Join(tempDir, "Test.xcresult")

	testParams := models.XcodebuildTestParams{
		BuildParams:                    buildParams,
		TestPlan:                       cfg.TestPlan,
		TestOutputDir:                  xcresultPath,
		TestRepetitionMode:             cfg.TestRepetitionMode,
		MaximumTestRepetitions:         cfg.MaximumTestRepetitions,
		RelaunchTestsForEachRepetition: cfg.RelaunchTestForEachRepetition,
		BuildBeforeTest:                cfg.BuildBeforeTesting,
		GenerateCodeCoverage:           cfg.GenerateCodeCoverageFiles,
		RetryTestsOnFailure:            cfg.RetryTestsOnFailure,
		AdditionalOptions:              cfg.XcodebuildTestOptions,
	}

	if cfg.IsSingleBuild {
		testParams.CleanBuild = cfg.IsCleanBuild
	}

	var swiftPackagesPath string
	if cfg.XcodeMajorVersion >= 11 {
		var err error
		swiftPackagesPath, err = cache.SwiftPackagesPath(cfg.ProjectPath)
		if err != nil {
			return result, fmt.Errorf("failed to get Swift Packages path, error: %s", err)
		}
	}

	params := testRunParams{
		buildTestParams:                    testParams,
		outputTool:                         cfg.OutputTool,
		xcprettyOptions:                    cfg.XcprettyOptions,
		retryOnTestRunnerError:             true,
		retryOnSwiftPackageResolutionError: true,
		swiftPackagesPath:                  swiftPackagesPath,
		xcodeMajorVersion:                  cfg.XcodeMajorVersion,
	}
	testLog, exitCode, testErr := runTest(params)
	result.XcresultPath = xcresultPath
	result.XcodebuildTestLog = testLog

	if testErr != nil || cfg.OutputTool == xcodebuildTool {
		printLastLinesOfXcodebuildTestLog(testLog, testErr == nil)
	}

	if cfg.SimulatorDebug == always || (cfg.SimulatorDebug == onFailure && testErr != nil) {
		fmt.Println()
		log.Infof("Collecting Simulator diagnostics")

		diagnosticsPath, err := simulatorCollectDiagnostics()
		if err != nil {
			log.Warnf("%v", err)
		} else {
			log.Donef("Simulator diagnistics are available as an artifact (%s)", diagnosticsPath)
			result.SimulatorDiagnosticsPath = diagnosticsPath
		}
	}

	// Shut down the simulator if it was started by the step for diagnostic logs.
	if !cfg.IsSimulatorBooted && cfg.SimulatorDebug != never {
		if err := simulatorShutdown(cfg.SimulatorID); err != nil {
			log.Warnf("%v", err)
		}
	}

	if testErr != nil {
		fmt.Println()
		log.Warnf("Xcode Test command exit code: %d", exitCode)
		log.Errorf("Xcode Test command failed, error: %s", testErr)
		return result, testErr
	}

	// Cache swift PM
	if cfg.XcodeMajorVersion >= 11 && cfg.CacheLevel == "swift_packages" {
		if err := cache.CollectSwiftPackages(cfg.ProjectPath); err != nil {
			log.Warnf("Failed to mark swift packages for caching, error: %s", err)
		}
	}

	fmt.Println()
	log.Infof("Xcode Test command succeeded.")

	return result, nil
}

// ExportOpts ...
type ExportOpts struct {
	TestFailed bool

	Scheme       string
	DeployDir    string
	XcresultPath string

	XcodebuildBuildLog string
	XcodebuildTestLog  string

	SimulatorDiagnosticsPath string
	ExportUITestArtifacts    bool
}

// Export ...
func (s Step) Export(opts ExportOpts) error {
	// export test run status
	status := "succeeded"
	if opts.TestFailed {
		status = "failed"
	}
	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", status); err != nil {
		log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
	}

	if opts.XcresultPath != "" {
		// export xcresult bundle
		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCRESULT_PATH", opts.XcresultPath); err != nil {
			log.Warnf("Failed to export: BITRISE_XCRESULT_PATH, error: %s", err)
		}

		xcresultZipPath := filepath.Join(opts.DeployDir, filepath.Base(opts.XcresultPath)+".zip")
		if err := output.ZipAndExportOutput(opts.XcresultPath, xcresultZipPath, "BITRISE_XCRESULT_ZIP_PATH"); err != nil {
			log.Warnf("Failed to export: BITRISE_XCRESULT_ZIP_PATH, error: %s", err)
		}

		// export xcresult for the testing addon
		if addonResultPath := os.Getenv(bitriseConfigs.BitrisePerStepTestResultDirEnvKey); len(addonResultPath) > 0 {
			fmt.Println()
			log.Infof("Exporting test results")

			if err := copyAndSaveMetadata(addonCopy{
				sourceTestOutputDir:   opts.XcresultPath,
				targetAddonPath:       addonResultPath,
				targetAddonBundleName: opts.Scheme,
			}); err != nil {
				log.Warnf("Failed to export test results, error: %s", err)
			}
		}
	}

	// export xcodebuild build log
	if opts.XcodebuildBuildLog != "" {
		pth, err := saveRawOutputToLogFile(opts.XcodebuildBuildLog)
		if err != nil {
			log.Warnf("Failed to save the Raw Output, err: %s", err)
		}

		deployPth := filepath.Join(opts.DeployDir, "xcodebuild_build.log")
		if err := command.CopyFile(pth, deployPth); err != nil {
			return fmt.Errorf("failed to copy xcodebuild output log file from (%s) to (%s), error: %s", pth, deployPth, err)
		}

		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODEBUILD_BUILD_LOG_PATH", deployPth); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODEBUILD_BUILD_LOG_PATH, error: %s", err)
		}
	}

	// export xcodebuild test log
	if opts.XcodebuildTestLog != "" {
		pth, err := saveRawOutputToLogFile(opts.XcodebuildTestLog)
		if err != nil {
			log.Warnf("Failed to save the Raw Output, error: %s", err)
		}

		deployPth := filepath.Join(opts.DeployDir, "xcodebuild_test.log")
		if err := command.CopyFile(pth, deployPth); err != nil {
			return fmt.Errorf("failed to copy xcodebuild output log file from (%s) to (%s), error: %s", pth, deployPth, err)
		}

		if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODEBUILD_TEST_LOG_PATH", deployPth); err != nil {
			log.Warnf("Failed to export: BITRISE_XCODEBUILD_TEST_LOG_PATH, error: %s", err)
		}
	}

	// export simulator diagnostics log
	if opts.SimulatorDiagnosticsPath != "" {
		diagnosticsName, err := simulatorDiagnosticsName()
		if err != nil {
			return err
		}

		outputPath := filepath.Join(opts.DeployDir, diagnosticsName)
		if err := ziputil.ZipDir(opts.SimulatorDiagnosticsPath, outputPath, true); err != nil {
			return fmt.Errorf("failed to compress simulator diagnostics result: %v", err)
		}
	}

	// export UITest artifacts
	if opts.ExportUITestArtifacts && opts.XcresultPath != "" {
		// The test result bundle (xcresult) structure changed in Xcode 11:
		// it does not contains TestSummaries.plist nor Attachments directly.
		fmt.Println()
		log.Infof("Exporting attachments")

		testSummariesPath, attachementDir, err := getSummariesAndAttachmentPath(opts.XcresultPath)
		if err != nil {
			log.Warnf("Failed to export UI test artifacts, error: %s", err)
		}

		if err := saveAttachments(opts.Scheme, testSummariesPath, attachementDir); err != nil {
			log.Warnf("Failed to export UI test artifacts, error: %s", err)
		}
	}

	return nil
}

func run() error {
	step := NewStep()
	config, err := step.ProcessConfig()
	if err != nil {
		return err
	}

	if err := step.InstallDeps(config.OutputTool == xcprettyTool); err != nil {
		config.OutputTool = xcodebuildTool
	}

	res, runErr := step.Run(config)

	opts := ExportOpts{
		TestFailed: runErr != nil,

		Scheme:       config.Scheme,
		DeployDir:    config.DeployDir,
		XcresultPath: res.XcresultPath,

		XcodebuildBuildLog: res.XcodebuildBuildLog,
		XcodebuildTestLog:  res.XcodebuildTestLog,

		SimulatorDiagnosticsPath: res.SimulatorDiagnosticsPath,
		ExportUITestArtifacts:    config.ExportUITestArtifacts,
	}
	exportErr := step.Export(opts)

	if runErr != nil {
		return runErr
	}

	return exportErr
}

func main() {
	if err := run(); err != nil {
		log.Errorf("Step run failed: %s", err.Error())
		os.Exit(1)
	}
}
