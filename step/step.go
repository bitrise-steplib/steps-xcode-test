package step

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-steplib/steps-xcode-test/cache"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/simulator"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/bitrise-steplib/steps-xcode-test/xcpretty"
)

const (
	minSupportedXcodeMajorVersion = 6
	simulatorShutdownState        = "Shutdown"
)

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

type exportCondition string

const (
	always    = "always"
	never     = "never"
	onFailure = "on_failure"
)

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

	SimulatorDebug exportCondition

	DeployDir             string
	ExportUITestArtifacts bool

	CacheLevel string
}

// XcodeTestRunner ...
type XcodeTestRunner struct {
	inputParser       stepconf.InputParser
	logger            log.Logger
	xcprettyInstaller xcpretty.Installer
	xcodebuild        xcodebuild.Xcodebuild
	simulatorManager  simulator.Manager
	cache             cache.SwiftPackageCache
	outputExporter    output.Exporter
	pathModifier      pathutil.PathModifier
	pathProvider      pathutil.PathProvider
}

// NewXcodeTestRunner ...
func NewXcodeTestRunner(inputParser stepconf.InputParser, logger log.Logger, xcprettyInstaller xcpretty.Installer, xcodebuild xcodebuild.Xcodebuild, simulatorManager simulator.Manager, cache cache.SwiftPackageCache, outputExporter output.Exporter, pathModifier pathutil.PathModifier, pathProvider pathutil.PathProvider) XcodeTestRunner {
	return XcodeTestRunner{
		inputParser:       inputParser,
		logger:            logger,
		xcprettyInstaller: xcprettyInstaller,
		xcodebuild:        xcodebuild,
		simulatorManager:  simulatorManager,
		cache:             cache,
		outputExporter:    outputExporter,
		pathModifier:      pathModifier,
		pathProvider:      pathProvider,
	}
}

// ProcessConfig ...
func (s XcodeTestRunner) ProcessConfig() (Config, error) {
	var input Input
	err := s.inputParser.Parse(&input)
	if err != nil {
		return Config{}, err
	}

	stepconf.Print(input)
	s.logger.Println()

	s.logger.EnableDebugLog(input.Verbose)

	// validate Xcode version
	xcodebuildVersion, err := s.xcodebuild.Version()
	if err != nil {
		return Config{}, fmt.Errorf("failed to determine Xcode version, error: %s", err)
	}
	s.logger.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	if err := s.validateXcodeVersion(&input, int(xcodebuildVersion.MajorVersion)); err != nil {
		return Config{}, err
	}

	// validate project path
	projectPath, err := s.pathModifier.AbsPath(input.ProjectPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute project path, error: %s", err)
	}
	if filepath.Ext(projectPath) != ".xcodeproj" && filepath.Ext(projectPath) != ".xcworkspace" {
		return Config{}, fmt.Errorf("invalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", projectPath)
	}

	// validate simulator related inputs
	sim, err := s.validateSimulator(input)
	if err != nil {
		return Config{}, err
	}

	// validate test repetition related inputs
	if input.TestRepetitionMode != xcodebuild.TestRepetitionNone && input.MaximumTestRepetitions < 2 {
		return Config{}, fmt.Errorf("invalid number of Maximum Test Repetitions (maximum_test_repetitions): %d, should be more than 1", input.MaximumTestRepetitions)
	}

	if input.RelaunchTestsForEachRepetition && input.TestRepetitionMode == xcodebuild.TestRepetitionNone {
		return Config{}, errors.New("Relaunch Tests for Each Repetition (relaunch_tests_for_each_repetition) cannot be used if Test Repetition Mode (test_repetition_mode) is 'none'")
	}

	return createConfig(input, projectPath, int(xcodebuildVersion.MajorVersion), sim), nil
}

// InstallDeps ...
func (s XcodeTestRunner) InstallDeps(xcpretty bool) error {
	if !xcpretty {
		return nil
	}

	xcprettyVersion, err := s.xcprettyInstaller.Install()
	if err != nil {
		return fmt.Errorf("an error occured during installing xcpretty: %s", err)
	}
	s.logger.Printf("- xcprettyVersion: %s", xcprettyVersion.String())
	s.logger.Println()

	return nil
}

// Result ...
type Result struct {
	Scheme                string
	DeployDir             string
	ExportUITestArtifacts bool

	XcresultPath             string
	XcodebuildBuildLog       string
	XcodebuildTestLog        string
	SimulatorDiagnosticsPath string
}

// Run ...
func (s XcodeTestRunner) Run(cfg Config) (Result, error) {
	enableSimulatorVerboseLog := cfg.SimulatorDebug != never
	launchSimulator := !cfg.IsSimulatorBooted && !cfg.HeadlessMode
	if err := s.prepareSimulator(enableSimulatorVerboseLog, cfg.SimulatorID, launchSimulator, cfg.XcodeMajorVersion); err != nil {
		return Result{}, err
	}

	var testErr error
	var testExitCode int
	result, code, err := s.runTests(cfg)
	if err != nil {
		if code == -1 {
			return result, err
		}

		testErr = err
		testExitCode = code
	}

	result.SimulatorDiagnosticsPath = s.teardownSimulator(cfg.SimulatorID, cfg.SimulatorDebug, cfg.IsSimulatorBooted, testErr)

	if testErr != nil {
		s.logger.Println()
		s.logger.Warnf("Xcode Test command exit code: %d", testExitCode)
		s.logger.Errorf("Xcode Test command failed, error: %s", testErr)
		return result, testErr
	}

	// Cache swift PM
	if cfg.XcodeMajorVersion >= 11 && cfg.CacheLevel == "swift_packages" {
		if err := s.cache.CollectSwiftPackages(cfg.ProjectPath); err != nil {
			s.logger.Warnf("Failed to mark swift packages for caching, error: %s", err)
		}
	}

	s.logger.Println()
	s.logger.Infof("Xcode Test command succeeded.")

	return result, nil
}

// Export ...
func (s XcodeTestRunner) Export(result Result, testFailed bool) error {
	// export test run status
	s.outputExporter.ExportTestRunResult(testFailed)

	if result.XcresultPath != "" {
		s.outputExporter.ExportXCResultBundle(result.DeployDir, result.XcresultPath, result.Scheme)
	}

	// export xcodebuild build log
	if result.XcodebuildBuildLog != "" {
		if err := s.outputExporter.ExportXcodebuildBuildLog(result.DeployDir, result.XcodebuildBuildLog); err != nil {
			return err
		}
	}

	// export xcodebuild test log
	if result.XcodebuildTestLog != "" {
		if err := s.outputExporter.ExportXcodebuildTestLog(result.DeployDir, result.XcodebuildTestLog); err != nil {
			return err
		}
	}

	// export simulator diagnostics log
	if result.SimulatorDiagnosticsPath != "" {
		diagnosticsName, err := s.simulatorManager.SimulatorDiagnosticsName()
		if err != nil {
			return err
		}

		if err := s.outputExporter.ExportSimulatorDiagnostics(result.DeployDir, result.SimulatorDiagnosticsPath, diagnosticsName); err != nil {
			return err
		}
	}

	// export UITest artifacts
	if result.ExportUITestArtifacts && result.XcresultPath != "" {
		s.outputExporter.ExportUITestArtifacts(result.XcresultPath, result.Scheme)
	}

	return nil
}

func (s XcodeTestRunner) validateXcodeVersion(input *Input, xcodeMajorVersion int) error {
	if xcodeMajorVersion < minSupportedXcodeMajorVersion {
		return fmt.Errorf("invalid Xcode major version (%d), should not be less then min supported: %d", xcodeMajorVersion, minSupportedXcodeMajorVersion)
	}

	if xcodeMajorVersion < 11 && input.TestPlan != "" {
		return fmt.Errorf("input Test Plan incompatible with Xcode %d, at least Xcode 11 required", xcodeMajorVersion)
	}

	// validate headless mode
	if xcodeMajorVersion < 9 && input.HeadlessMode {
		s.logger.Warnf("Headless mode is enabled but it's only available with Xcode 9.x or newer.")
		input.HeadlessMode = false
	}

	// validate export UITest artifacts
	if input.ExportUITestArtifacts && xcodeMajorVersion >= 11 {
		// The test result bundle (xcresult) structure changed in Xcode 11:
		// it does not contains TestSummaries.plist nor Attachments directly.
		s.logger.Warnf("Export UITest Artifacts (export_uitest_artifacts) turned on, but Xcode version >= 11. The test result bundle structure changed in Xcode 11 it does not contain TestSummaries.plist and Attachments directly, nothing to export.")
		input.ExportUITestArtifacts = false
	}

	// validate simulator diagnosis mode
	if input.CollectSimulatorDiagnostics != never && xcodeMajorVersion < 10 {
		s.logger.Warnf("Collecting Simulator diagnostics is not available below Xcode version 10, current Xcode version: %s", xcodeMajorVersion)
		input.CollectSimulatorDiagnostics = never
	}

	if input.TestRepetitionMode != xcodebuild.TestRepetitionNone && xcodeMajorVersion < 13 {
		return errors.New("Test Repetition Mode (test_repetition_mode) is not available below Xcode 13")
	}

	if input.RetryTestsOnFailure && xcodeMajorVersion > 12 {
		return errors.New("Should retry tests on failure? (should_retry_test_on_fail) is not available above Xcode 12; use test_repetition_mode=retry_on_failure instead")
	}

	return nil
}

func (s XcodeTestRunner) validateSimulator(input Input) (simulator.Simulator, error) {
	var sim simulator.Simulator
	var osVersion string

	platform := strings.TrimSuffix(input.SimulatorPlatform, " Simulator")
	// Retry gathering device information since xcrun simctl list can fail to show the complete device list
	if err := retry.Times(3).Wait(10 * time.Second).Try(func(attempt uint) error {
		var errGetSimulator error
		if input.SimulatorOsVersion == "latest" {
			var simulatorDevice = input.SimulatorDevice
			if simulatorDevice == "iPad" {
				s.logger.Warnf("Given device (%s) is deprecated, using iPad Air (3rd generation)...", simulatorDevice)
				simulatorDevice = "iPad Air (3rd generation)"
			}

			sim, osVersion, errGetSimulator = s.simulatorManager.GetLatestSimulatorAndVersion(platform, simulatorDevice)
		} else {
			normalizedOsVersion := input.SimulatorOsVersion
			osVersionSplit := strings.Split(normalizedOsVersion, ".")
			if len(osVersionSplit) > 2 {
				normalizedOsVersion = strings.Join(osVersionSplit[0:2], ".")
			}
			osVersion = fmt.Sprintf("%s %s", platform, normalizedOsVersion)

			sim, errGetSimulator = s.simulatorManager.GetSimulator(osVersion, input.SimulatorDevice)
		}

		if errGetSimulator != nil {
			s.logger.Warnf("attempt %d to get simulator udid failed with error: %s", attempt, errGetSimulator)
		}

		return errGetSimulator
	}); err != nil {
		return simulator.Simulator{}, fmt.Errorf("simulator UDID lookup failed: %s", err)
	}

	s.logger.Infof("Simulator infos")
	s.logger.Printf("* simulator_name: %s, version: %s, UDID: %s, status: %s", sim.Name, osVersion, sim.ID, sim.Status)

	return sim, nil
}

func (s XcodeTestRunner) prepareSimulator(enableSimulatorVerboseLog bool, simulatorID string, launchSimulator bool, xcodeMajorVersions int) error {
	err := s.simulatorManager.ResetLaunchServices()
	if err != nil {
		s.logger.Warnf("Failed to apply simulator boot workaround, error: %s", err)
	}

	// Boot simulator
	if enableSimulatorVerboseLog {
		s.logger.Infof("Enabling Simulator verbose log for better diagnostics")
		// Boot the simulator now, so verbose logging can be enabled and it is kept booted after running tests,
		// this helps to collect more detailed debug info
		if err := s.simulatorManager.SimulatorBoot(simulatorID); err != nil {
			return fmt.Errorf("%v", err)
		}
		if err := s.simulatorManager.SimulatorEnableVerboseLog(simulatorID); err != nil {
			return fmt.Errorf("%v", err)
		}

		s.logger.Println()
	}

	if launchSimulator {
		s.logger.Infof("Booting simulator (%s)...", simulatorID)

		if err := s.simulatorManager.LaunchSimulator(simulatorID, xcodeMajorVersions); err != nil {
			return fmt.Errorf("failed to boot simulator, error: %s", err)
		}

		progress.NewDefaultWrapper("Waiting for simulator boot").WrapAction(func() {
			time.Sleep(60 * time.Second)
		})

		s.logger.Println()
	}

	return nil
}

func (s XcodeTestRunner) runTests(cfg Config) (Result, int, error) {
	// Run build
	result := Result{
		Scheme:                cfg.Scheme,
		DeployDir:             cfg.DeployDir,
		ExportUITestArtifacts: cfg.ExportUITestArtifacts,
	}

	buildParams := createBuildParams(cfg)

	if !cfg.IsSingleBuild {
		buildLog, exitCode, err := s.xcodebuild.RunBuild(buildParams, cfg.OutputTool)
		result.XcodebuildBuildLog = buildLog
		if err != nil {
			s.logger.Warnf("xcode build exit code: %d", exitCode)
			s.logger.Warnf("xcode build log:\n%s", buildLog)
			s.logger.Errorf("xcode build failed with error: %s", err)
			return result, -1, err
		}
	}

	// Run test
	tempDir, err := s.pathProvider.CreateTempDir("XCUITestOutput")
	if err != nil {
		return result, -1, fmt.Errorf("could not create test output temporary directory: %s", err)
	}
	xcresultPath := path.Join(tempDir, "Test.xcresult")

	var swiftPackagesPath string
	if cfg.XcodeMajorVersion >= 11 {
		var err error
		swiftPackagesPath, err = s.cache.SwiftPackagesPath(cfg.ProjectPath)
		if err != nil {
			return result, -1, fmt.Errorf("failed to get Swift Packages path, error: %s", err)
		}
	}

	testParams := createTestParams(cfg, buildParams, xcresultPath, swiftPackagesPath)

	testLog, exitCode, testErr := s.xcodebuild.RunTest(testParams)
	result.XcresultPath = xcresultPath
	result.XcodebuildTestLog = testLog

	if testErr != nil || cfg.OutputTool == xcodebuild.XcodebuildTool {
		printLastLinesOfXcodebuildTestLog(testLog, testErr == nil)
	}

	return result, exitCode, testErr
}

func (s XcodeTestRunner) teardownSimulator(simulatorID string, simulatorDebug exportCondition, isSimulatorBooted bool, testErr error) string {
	var simulatorDiagnosticsPath string

	if simulatorDebug == always || (simulatorDebug == onFailure && testErr != nil) {
		s.logger.Println()
		s.logger.Infof("Collecting Simulator diagnostics")

		diagnosticsPath, err := s.simulatorManager.SimulatorCollectDiagnostics()
		if err != nil {
			s.logger.Warnf("%v", err)
		} else {
			s.logger.Donef("Simulator diagnostics are available as an artifact (%s)", diagnosticsPath)
			simulatorDiagnosticsPath = diagnosticsPath
		}
	}

	// Shut down the simulator if it was started by the step for diagnostic logs.
	if !isSimulatorBooted && simulatorDebug != never {
		if err := s.simulatorManager.SimulatorShutdown(simulatorID); err != nil {
			s.logger.Warnf("%v", err)
		}
	}

	return simulatorDiagnosticsPath
}

func createConfig(input Input, projectPath string, xcodeMajorVersion int, sim simulator.Simulator) Config {
	return Config{
		ProjectPath: projectPath,
		Scheme:      input.Scheme,
		TestPlan:    input.TestPlan,

		XcodeMajorVersion: xcodeMajorVersion,
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
		HeadlessMode:              input.HeadlessMode,

		XcodebuildTestOptions: input.TestOptions,
		XcprettyOptions:       input.XcprettyTestOptions,

		SimulatorDebug: exportCondition(input.CollectSimulatorDiagnostics),

		DeployDir:             input.DeployDir,
		ExportUITestArtifacts: input.ExportUITestArtifacts,

		CacheLevel: input.CacheLevel,
	}
}

func createBuildParams(cfg Config) xcodebuild.Params {
	projectFlag := "-project"
	if filepath.Ext(cfg.ProjectPath) == ".xcworkspace" {
		projectFlag = "-workspace"
	}

	return xcodebuild.Params{
		Action:                    projectFlag,
		ProjectPath:               cfg.ProjectPath,
		Scheme:                    cfg.Scheme,
		DeviceDestination:         fmt.Sprintf("id=%s", cfg.SimulatorID),
		CleanBuild:                cfg.IsCleanBuild,
		DisableIndexWhileBuilding: cfg.DisableIndexWhileBuilding,
	}
}

func createTestParams(cfg Config, buildParams xcodebuild.Params, xcresultPath, swiftPackagesPath string) xcodebuild.TestRunParams {
	testParams := xcodebuild.TestParams{
		BuildParams:                    buildParams,
		TestPlan:                       cfg.TestPlan,
		TestOutputDir:                  xcresultPath,
		TestRepetitionMode:             cfg.TestRepetitionMode,
		MaximumTestRepetitions:         cfg.MaximumTestRepetitions,
		RelaunchTestsForEachRepetition: cfg.RelaunchTestForEachRepetition,
		CleanBuild:                     cfg.IsCleanBuild,
		BuildBeforeTest:                cfg.BuildBeforeTesting,
		GenerateCodeCoverage:           cfg.GenerateCodeCoverageFiles,
		RetryTestsOnFailure:            cfg.RetryTestsOnFailure,
		AdditionalOptions:              cfg.XcodebuildTestOptions,
	}

	return xcodebuild.TestRunParams{
		BuildTestParams:                    testParams,
		OutputTool:                         cfg.OutputTool,
		XcprettyOptions:                    cfg.XcprettyOptions,
		RetryOnTestRunnerError:             true,
		RetryOnSwiftPackageResolutionError: true,
		SwiftPackagesPath:                  swiftPackagesPath,
		XcodeMajorVersion:                  cfg.XcodeMajorVersion,
	}
}
