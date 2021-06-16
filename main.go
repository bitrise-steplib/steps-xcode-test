package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	bitriseConfigs "github.com/bitrise-io/bitrise/configs"
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/retry"
	simulator "github.com/bitrise-io/go-xcode/simulator"
	"github.com/bitrise-io/go-xcode/utility"
	cache "github.com/bitrise-io/go-xcode/xcodecache"
	cmd "github.com/bitrise-steplib/steps-xcode-test/command"
	"github.com/bitrise-steplib/steps-xcode-test/models"
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

	xcodeBuild             = "xcodebuild"
	simulatorShutdownState = "Shutdown"
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

type Step struct{}

func NewStep() Step {
	return Step{}
}

// Input ...
type Input struct {
	// Project Parameters
	ProjectPath string `env:"project_path,required"`
	Scheme      string `env:"scheme,required"`

	// Simulator Configs
	SimulatorPlatform  string `env:"simulator_platform,required"`
	SimulatorDevice    string `env:"simulator_device,required"`
	SimulatorOsVersion string `env:"simulator_os_version,required"`

	// Test Run Configs
	OutputTool            string `env:"output_tool,opt[xcpretty,xcodebuild]"`
	IsCleanBuild          bool   `env:"is_clean_build,opt[yes,no]"`
	IsSingleBuild         bool   `env:"single_build,opt[true,false]"`
	ShouldBuildBeforeTest bool   `env:"should_build_before_test,opt[yes,no]"`

	ShouldRetryTestOnFail     bool `env:"should_retry_test_on_fail,opt[yes,no]"`
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

type Config struct {
	ProjectPath string
	Scheme      string

	XcodeMajorVersion int
	SimulatorID       string
	IsSimulatorBooted bool

	OutputTool         string
	IsCleanBuild       bool
	IsSingleBuild      bool
	BuildBeforeTesting bool

	ShouldRetryTestOnFail     bool
	DisableIndexWhileBuilding bool
	GenerateCodeCoverageFiles bool
	HeadlessMode              bool

	XcodebuildTestoptions string
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
		return Config{}, fmt.Errorf("failed to determine xcode version, error: %s", err)
	}
	log.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	xcodeMajorVersion := xcodebuildVersion.MajorVersion
	if xcodeMajorVersion < minSupportedXcodeMajorVersion {
		return Config{}, fmt.Errorf("invalid xcode major version (%d), should not be less then min supported: %d", xcodeMajorVersion, minSupportedXcodeMajorVersion)
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
	var (
		sim       simulator.InfoModel
		osVersion string
	)

	platform := strings.TrimSuffix(input.SimulatorPlatform, " Simulator")
	// Retry gathering device information since xcrun simctl list can fail to show the complete device list
	if err = retry.Times(3).Wait(10 * time.Second).Try(func(attempt uint) error {
		var errGetSimulator error
		if input.SimulatorOsVersion == "latest" {
			var simulatorDevice = input.SimulatorDevice
			if simulatorDevice == "iPad" {
				// TODO: missleading log
				log.Warnf("Given device (%s) is deprecated, using (iPad 2)...", simulatorDevice)
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
		// if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
		// 	log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		// }

		return Config{}, fmt.Errorf("simulator UDID lookup failed: %s", err)
	}

	log.Infof("Simulator infos")
	log.Printf("* simulator_name: %s, version: %s, UDID: %s, status: %s", sim.Name, osVersion, sim.ID, sim.Status)

	// Device Destination
	deviceDestination := fmt.Sprintf("id=%s", sim.ID)

	log.Printf("* device_destination: %s", deviceDestination)
	fmt.Println()

	return Config{
		ProjectPath: projectPath,
		Scheme:      input.Scheme,

		XcodeMajorVersion: int(xcodeMajorVersion),
		SimulatorID:       sim.ID,
		IsSimulatorBooted: sim.Status != simulatorShutdownState,

		OutputTool:         input.OutputTool,
		IsCleanBuild:       input.IsCleanBuild,
		IsSingleBuild:      input.IsSingleBuild,
		BuildBeforeTesting: input.ShouldBuildBeforeTest,

		ShouldRetryTestOnFail:     input.ShouldRetryTestOnFail,
		DisableIndexWhileBuilding: input.DisableIndexWhileBuilding,
		GenerateCodeCoverageFiles: input.GenerateCodeCoverageFiles,
		HeadlessMode:              headlessMode,

		XcodebuildTestoptions: input.TestOptions,
		XcprettyOptions:       input.XcprettyTestOptions,

		Verbose:        input.Verbose,
		SimulatorDebug: simulatorDebug,

		DeployDir:             input.DeployDir,
		ExportUITestArtifacts: exportUITestArtifacts,

		CacheLevel: input.CacheLevel,
	}, nil
}

type Result struct {
	TestOutputDir      string
	XcodebuildBuildLog string
	XcodebuildTestLog  string
}

func (s Step) Run(cfg Config) (Result, error) {
	log.SetEnableDebugLog(cfg.Verbose)

	// Ensure xcpretty installed
	xcprettyVersion, err := InstallXcpretty()
	if err != nil {
		cfg.OutputTool, err = handleXcprettyInstallError(err)
		if err != nil {
			return Result{}, fmt.Errorf("an error occured during installing xcpretty: %s", err)
		}
	} else {
		log.Printf("- xcprettyVersion: %s", xcprettyVersion.String())
		fmt.Println()
	}

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

	buildParams := models.XcodeBuildParamsModel{
		Action:                    projectFlag,
		ProjectPath:               cfg.ProjectPath,
		Scheme:                    cfg.Scheme,
		DeviceDestination:         fmt.Sprintf("id=%s", cfg.SimulatorID),
		CleanBuild:                cfg.IsCleanBuild,
		DisableIndexWhileBuilding: cfg.DisableIndexWhileBuilding,
	}

	if !cfg.IsSingleBuild {
		buildLog, exitCode, buildErr := runBuild(buildParams, cfg.OutputTool)
		result.XcodebuildBuildLog = buildLog
		if buildErr != nil {
			log.Warnf("xcode build exit code: %d", exitCode)
			log.Warnf("xcode build log:\n%s", buildLog)
			log.Errorf("xcode build failed with error: %s", buildErr)
			return result, buildErr

			// TODO: move output export
			// if _, err := saveRawOutputToLogFile(rawXcodebuildOutput, false, false); err != nil {
			// 	log.Warnf("Failed to save the Raw Output, err: %s", err)
			// }

			// log.Warnf("xcode build exit code: %d", exitCode)
			// log.Warnf("xcode build log:\n%s", rawXcodebuildOutput)
			// log.Errorf("xcode build failed with error: %s", buildErr)
			// if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
			// 	log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
			// }
			// os.Exit(1)
		}
	}

	// Run test
	tempDir, err := ioutil.TempDir("", "XCUITestOutput")
	if err != nil {
		return result, fmt.Errorf("could not create test output temporary directory: %s", err)
	}
	// Leaving the output dir in place after exiting
	testOutputDir := path.Join(tempDir, "Test.xcresult")
	result.TestOutputDir = testOutputDir

	buildTestParams := models.XcodeBuildTestParamsModel{
		BuildParams:          buildParams,
		TestOutputDir:        testOutputDir,
		BuildBeforeTest:      cfg.BuildBeforeTesting,
		AdditionalOptions:    cfg.XcodebuildTestoptions,
		GenerateCodeCoverage: cfg.GenerateCodeCoverageFiles,
	}

	var swiftPackagesPath string
	if cfg.XcodeMajorVersion >= 11 {
		var err error
		swiftPackagesPath, err = cache.SwiftPackagesPath(cfg.ProjectPath)
		if err != nil {
			return result, fmt.Errorf("failed to get Swift Packages path, error: %s", err)
		}
	}

	testLog, exitCode, testErr := runTest(buildTestParams, cfg.OutputTool, cfg.XcprettyOptions, true, cfg.ShouldRetryTestOnFail, swiftPackagesPath)
	result.XcodebuildTestLog = testLog
	if testErr != nil {
		return result, err
	}

	if testErr != nil || cfg.OutputTool == xcodeBuild {
		printLastLinesOfRawXcodebuildLog(testLog, testErr == nil)
	}

	if cfg.SimulatorDebug == always || (cfg.SimulatorDebug == onFailure && testErr != nil) {
		// Shut down Simulator if it was not booted initially
		if !cfg.IsSimulatorBooted {
			if err := simulatorShutdown(cfg.SimulatorID); err != nil {
				log.Warnf("%v", err)
			}
		}
	}

	if testErr != nil {
		fmt.Println()
		log.Warnf("Xcode Test command exit code: %d", exitCode)
		log.Errorf("Xcode Test command failed, error: %s", testErr)

		// if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCODE_TEST_RESULT", "failed"); err != nil {
		// 	log.Warnf("Failed to export: BITRISE_XCODE_TEST_RESULT, error: %s", err)
		// }
		// os.Exit(1)
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

type ExportOpts struct {
	TestFailed bool

	Scheme        string
	DeployDir     string
	TestResultDir string

	XcodebuildBuildLog string
	XcodebuildTestLog  string

	CollectSimulatorDiagnoscitcs bool
	ExportUITestArtifacts        bool
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

	// export xcresult bundle
	if err := cmd.ExportEnvironmentWithEnvman("BITRISE_XCRESULT_PATH", opts.TestResultDir); err != nil {
		log.Warnf("Failed to export: BITRISE_XCRESULT_PATH, error: %s", err)
	}

	// exporting xcresult only if test result dir is present
	if addonResultPath := os.Getenv(bitriseConfigs.BitrisePerStepTestResultDirEnvKey); len(addonResultPath) > 0 {
		fmt.Println()
		log.Infof("Exporting test results")

		if err := copyAndSaveMetadata(addonCopy{
			sourceTestOutputDir:   opts.TestResultDir,
			targetAddonPath:       addonResultPath,
			targetAddonBundleName: opts.Scheme,
		}); err != nil {
			log.Warnf("Failed to export test results, error: %s", err)
		}
	}

	if opts.XcodebuildBuildLog != "" {
		if _, err := saveRawOutputToLogFile(opts.XcodebuildBuildLog, false, false); err != nil {
			log.Warnf("Failed to save the Raw Output, err: %s", err)
		}

	}

	if opts.XcodebuildTestLog != "" {
		_, err := saveRawOutputToLogFile(opts.XcodebuildTestLog, false, false)
		if err != nil {
			log.Warnf("Failed to save the Raw Output, error: %s", err)
		}
	}

	if opts.CollectSimulatorDiagnoscitcs {
		fmt.Println()
		log.Infof("Collecting Simulator diagnostics")
		if opts.DeployDir != "" {
			diagnosticsPath, err := simulatorCollectDiagnostics(opts.DeployDir)
			if err != nil {
				log.Warnf("%v", err)
			} else {
				log.Donef("Simulator diagnistics are available as an artifact (%s)", diagnosticsPath)
			}
		} else {
			log.Warnf("No deploy directory specified, will not export Simulator diagnostics")
		}
	}

	if opts.ExportUITestArtifacts {
		// The test result bundle (xcresult) structure changed in Xcode 11:
		// it does not contains TestSummaries.plist nor Attachments directly.
		fmt.Println()
		log.Infof("Exporting attachments")

		testSummariesPath, attachementDir, err := getSummariesAndAttachmentPath(opts.TestResultDir)
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

	res, stepRunErr := step.Run(config)

	// TODO: differentiate xcodebuild test error from other errors
	collectSimulatorDiagnostics := config.SimulatorDebug == always || (config.SimulatorDebug == onFailure && stepRunErr != nil)
	opts := ExportOpts{
		TestFailed:                   stepRunErr != nil,
		Scheme:                       config.Scheme,
		DeployDir:                    config.DeployDir,
		TestResultDir:                res.TestOutputDir,
		XcodebuildBuildLog:           res.XcodebuildBuildLog,
		XcodebuildTestLog:            res.XcodebuildTestLog,
		CollectSimulatorDiagnoscitcs: collectSimulatorDiagnostics,
		ExportUITestArtifacts:        config.ExportUITestArtifacts,
	}
	if err := step.Export(opts); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Errorf("Step run failed: %s", err.Error())
		os.Exit(1)
	}
}
