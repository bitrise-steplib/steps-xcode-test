package step

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/destination"
	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
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
	ProjectPath string `env:"project_path,required"`
	Scheme      string `env:"scheme,required"`
	Destination string `env:"destination,required"`
	TestPlan    string `env:"test_plan"`

	// Test Repetition
	TestRepetitionMode             string `env:"test_repetition_mode,opt[none,until_failure,retry_on_failure,up_until_maximum_repetitions]"`
	MaximumTestRepetitions         int    `env:"maximum_test_repetitions,required"`
	RelaunchTestsForEachRepetition bool   `env:"relaunch_tests_for_each_repetition,opt[yes,no]"`
	RetryTestsOnFailure            bool   `env:"should_retry_test_on_fail,opt[yes,no]"`

	// xcodebuild configuration
	XCConfigContent    string `env:"xcconfig_content"`
	PerformCleanAction bool   `env:"perform_clean_action,opt[yes,no]"`
	XcodebuildOptions  string `env:"xcodebuild_options"`

	// xcodebuild log formatting
	LogFormatter    string `env:"log_formatter,opt[xcpretty,xcodebuild]"`
	XcprettyOptions string `env:"xcpretty_options"`

	// Caching
	CacheLevel string `env:"cache_level,opt[none,swift_packages]"`

	// Debugging
	VerboseLog                  bool   `env:"verbose_log,opt[yes,no]"`
	CollectSimulatorDiagnostics string `env:"collect_simulator_diagnostics,opt[always,on_failure,never]"`
	HeadlessMode                bool   `env:"headless_mode,opt[yes,no]"`

	// Output export
	DeployDir string `env:"BITRISE_DEPLOY_DIR"`
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

	SimulatorID       string
	IsSimulatorBooted bool

	XcodeMajorVersion int

	TestRepetitionMode            string
	MaximumTestRepetitions        int
	RelaunchTestForEachRepetition bool
	RetryTestsOnFailure           bool

	XCConfigContent    string
	PerformCleanAction bool
	XcodebuildOptions  string

	LogFormatter    string
	XcprettyOptions string

	CacheLevel string

	CollectSimulatorDiagnostics exportCondition
	HeadlessMode                bool

	DeployDir string
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

	s.logger.EnableDebugLog(input.VerboseLog)

	// validate Xcode version
	xcodebuildVersion, err := s.xcodebuild.Version()
	if err != nil {
		return Config{}, fmt.Errorf("failed to determine Xcode version: %w", err)
	}
	s.logger.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	if err := s.validateXcodeVersion(&input, int(xcodebuildVersion.MajorVersion)); err != nil {
		return Config{}, err
	}

	// validate project path
	projectPath, err := s.pathModifier.AbsPath(input.ProjectPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute project path: %w", err)
	}
	if filepath.Ext(projectPath) != ".xcodeproj" && filepath.Ext(projectPath) != ".xcworkspace" {
		return Config{}, fmt.Errorf("invalid project file (%s), extension should be (.xcodeproj/.xcworkspace)", projectPath)
	}

	sim, err := s.getSimulatorForDestination(input.Destination)
	if err != nil {
		return Config{}, err
	}

	// validate test repetition related inputs
	if input.TestRepetitionMode != xcodebuild.TestRepetitionNone && input.MaximumTestRepetitions < 2 {
		return Config{}, fmt.Errorf("invalid number of Maximum Test Repetitions (maximum_test_repetitions): %d, should be more than 1", input.MaximumTestRepetitions)
	}

	if input.RelaunchTestsForEachRepetition && input.TestRepetitionMode == xcodebuild.TestRepetitionNone {
		return Config{}, errors.New("the Relaunch Tests for Each Repetition (relaunch_tests_for_each_repetition) cannot be used if Test Repetition Mode (test_repetition_mode) is 'none'")
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
		return fmt.Errorf("installing xcpretty: %w", err)
	}
	s.logger.Printf("- xcpretty version: %s", xcprettyVersion.String())
	s.logger.Println()

	return nil
}

// Result ...
type Result struct {
	Scheme    string
	DeployDir string

	XcresultPath             string
	XcodebuildBuildLog       string
	XcodebuildTestLog        string
	SimulatorDiagnosticsPath string
}

// Run ...
func (s XcodeTestRunner) Run(cfg Config) (Result, error) {
	enableSimulatorVerboseLog := cfg.CollectSimulatorDiagnostics != never
	launchSimulator := !cfg.IsSimulatorBooted && !cfg.HeadlessMode
	if err := s.prepareSimulator(enableSimulatorVerboseLog, cfg.SimulatorID, launchSimulator); err != nil {
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

	result.SimulatorDiagnosticsPath = s.teardownSimulator(cfg.SimulatorID, cfg.CollectSimulatorDiagnostics, cfg.IsSimulatorBooted, testErr)

	if testErr != nil {
		s.logger.Println()
		s.logger.Warnf("Xcode Test command exit code: %d", testExitCode)
		s.logger.Errorf("Xcode Test command failed: %s", testErr)
		return result, testErr
	}

	// Cache swift PM
	if cfg.XcodeMajorVersion >= 11 && cfg.CacheLevel == "swift_packages" {
		if err := s.cache.CollectSwiftPackages(cfg.ProjectPath); err != nil {
			s.logger.Warnf("Failed to mark swift packages for caching: %s", err)
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
			return fmt.Errorf("failed to get simulator diagnostics name: %w", err)
		}

		if err := s.outputExporter.ExportSimulatorDiagnostics(result.DeployDir, result.SimulatorDiagnosticsPath, diagnosticsName); err != nil {
			return fmt.Errorf("failed to export simulator diagnostics: %w", err)
		}
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

func (s XcodeTestRunner) getSimulatorForDestination(destinationSpecifier string) (simulator.Simulator, error) {
	var sim simulator.Simulator
	var osVersion string

	simulatorDestination, err := destination.NewSimulator(destinationSpecifier)
	if err != nil {
		return simulator.Simulator{}, fmt.Errorf("invalid destination specifier: %s: %w", destinationSpecifier, err)
	}

	platform := strings.TrimSuffix(simulatorDestination.Platform, " Simulator")
	// Retry gathering device information since xcrun simctl list can fail to show the complete device list
	if err := retry.Times(3).Wait(10 * time.Second).Try(func(attempt uint) error {
		var errGetSimulator error
		if simulatorDestination.OS == "latest" {
			simulatorDevice := simulatorDestination.Name
			if simulatorDevice == "iPad" {
				s.logger.Warnf("Given device (%s) is deprecated, using iPad Air (3rd generation)...", simulatorDevice)
				simulatorDevice = "iPad Air (3rd generation)"
			}

			sim, osVersion, errGetSimulator = s.simulatorManager.GetLatestSimulatorAndVersion(platform, simulatorDevice)
		} else {
			normalizedOsVersion := simulatorDestination.OS
			osVersionSplit := strings.Split(normalizedOsVersion, ".")
			if len(osVersionSplit) > 2 {
				normalizedOsVersion = strings.Join(osVersionSplit[0:2], ".")
			}
			osVersion = fmt.Sprintf("%s %s", platform, normalizedOsVersion)

			sim, errGetSimulator = s.simulatorManager.GetSimulator(osVersion, simulatorDestination.Name)
		}

		if errGetSimulator != nil {
			s.logger.Warnf("attempt %d to get simulator UDID failed with error: %s", attempt, errGetSimulator)
		}

		return errGetSimulator
	}); err != nil {
		return simulator.Simulator{}, fmt.Errorf("simulator UDID lookup failed: %w", err)
	}

	s.logger.Infof("Simulator infos")
	s.logger.Printf("* simulator_name: %s, version: %s, UDID: %s, status: %s", sim.Name, osVersion, sim.ID, sim.Status)

	return sim, nil
}

func (s XcodeTestRunner) prepareSimulator(enableSimulatorVerboseLog bool, simulatorID string, launchSimulator bool) error {
	err := s.simulatorManager.ResetLaunchServices()
	if err != nil {
		s.logger.Warnf("Failed to apply simulator boot workaround, error: %s", err)
	}

	// Boot simulator
	if enableSimulatorVerboseLog {
		s.logger.Infof("Enabling Simulator verbose log for better diagnostics")
		// Boot the simulator now, so verbose logging can be enabled, and it is kept booted after running tests.
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

		if err := s.simulatorManager.LaunchSimulator(simulatorID); err != nil {
			return fmt.Errorf("failed to boot simulator: %w", err)
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
		Scheme:    cfg.Scheme,
		DeployDir: cfg.DeployDir,
	}

	// Run test
	tempDir, err := s.pathProvider.CreateTempDir("XCUITestOutput")
	if err != nil {
		return result, -1, fmt.Errorf("could not create test output temporary directory: %w", err)
	}
	xcresultPath := path.Join(tempDir, fmt.Sprintf("Test-%s.xcresult", cfg.Scheme))

	var swiftPackagesPath string
	if cfg.XcodeMajorVersion >= 11 {
		var err error
		swiftPackagesPath, err = s.cache.SwiftPackagesPath(cfg.ProjectPath)
		if err != nil {
			return result, -1, fmt.Errorf("failed to get Swift Packages path: %w", err)
		}
	}

	testParams := createTestParams(cfg, xcresultPath, swiftPackagesPath)

	testLog, exitCode, testErr := s.xcodebuild.RunTest(testParams)
	result.XcresultPath = xcresultPath
	result.XcodebuildTestLog = testLog

	if testErr != nil || cfg.LogFormatter == xcodebuild.XcodebuildTool {
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
			s.logger.Warnf(err.Error())
		} else {
			s.logger.Donef("Simulator diagnostics are available as an artifact (%s)", diagnosticsPath)
			simulatorDiagnosticsPath = diagnosticsPath
		}
	}

	// Shut down the simulator if it was started by the step for diagnostic logs.
	if !isSimulatorBooted && simulatorDebug != never {
		if err := s.simulatorManager.SimulatorShutdown(simulatorID); err != nil {
			s.logger.Warnf(err.Error())
		}
	}

	return simulatorDiagnosticsPath
}
