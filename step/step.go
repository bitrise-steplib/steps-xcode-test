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
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-io/go-xcode/v2/simulator"
	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/bitrise-steplib/steps-xcode-test/xcodecommand"
	"github.com/bitrise-steplib/steps-xcode-test/xcodeversion"
	"github.com/kballard/go-shellquote"
)

const (
	minSupportedXcodeMajorVersion = 11
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
	LogFormatter        string `env:"log_formatter,opt[xcbeautify,xcodebuild,xcpretty]"`
	XcprettyOptions     string `env:"xcpretty_options"`
	LogFormatterOptions string `env:"log_formatter_options"`

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

// Output tools ...
const (
	XcbeautifyTool = "xcbeautify"
	XcodebuildTool = "xcodebuild"
	XcprettyTool   = "xcpretty"
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
	XcodebuildOptions  []string

	LogFormatter        string
	LogFormatterOptions []string

	CacheLevel string

	CollectSimulatorDiagnostics exportCondition
	HeadlessMode                bool

	DeployDir string
}

type XcodeTestConfigParser struct {
	logger             log.Logger
	inputParser        stepconf.InputParser
	xcodeVersionReader xcodeversion.Reader
	deviceFinder       destination.DeviceFinder
	pathModifier       pathutil.PathModifier
	utils              Utils
}

func NewXcodeTestConfigParser(inputParser stepconf.InputParser, logger log.Logger, xcodeVersionReader xcodeversion.Reader, deviceFinder destination.DeviceFinder, pathModifier pathutil.PathModifier, utils Utils) XcodeTestConfigParser {
	return XcodeTestConfigParser{
		logger:             logger,
		inputParser:        inputParser,
		xcodeVersionReader: xcodeVersionReader,
		deviceFinder:       deviceFinder,
		pathModifier:       pathModifier,
		utils:              utils,
	}
}

// XcodeTestRunner ...
type XcodeTestRunner struct {
	logger                log.Logger
	logFormatterInstaller xcodecommand.DependencyInstaller
	xcodebuild            xcodebuild.Xcodebuild
	simulatorManager      simulator.Manager
	cache                 cache.SwiftPackageCache
	outputExporter        output.Exporter
	pathModifier          pathutil.PathModifier
	pathProvider          pathutil.PathProvider
	utils                 Utils
}

// NewXcodeTestRunner ...
func NewXcodeTestRunner(logger log.Logger, logFormatterInstaller xcodecommand.DependencyInstaller, xcodebuild xcodebuild.Xcodebuild, simulatorManager simulator.Manager, cache cache.SwiftPackageCache, outputExporter output.Exporter, pathModifier pathutil.PathModifier, pathProvider pathutil.PathProvider, utils Utils) XcodeTestRunner {
	return XcodeTestRunner{
		logger:                logger,
		logFormatterInstaller: logFormatterInstaller,
		xcodebuild:            xcodebuild,
		simulatorManager:      simulatorManager,
		cache:                 cache,
		outputExporter:        outputExporter,
		pathModifier:          pathModifier,
		pathProvider:          pathProvider,
		utils:                 utils,
	}
}

// ProcessConfig ...
func (s XcodeTestConfigParser) ProcessConfig() (Config, error) {
	var input Input
	err := s.inputParser.Parse(&input)
	if err != nil {
		return Config{}, err
	}

	stepconf.Print(input)
	s.logger.Println()

	s.logger.EnableDebugLog(input.VerboseLog)

	// validate Xcode version
	xcodebuildVersion, err := s.xcodeVersionReader.Version()
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
	fileExtension := filepath.Ext(projectPath)
	if fileExtension != ".xcodeproj" && fileExtension != ".xcworkspace" && filepath.Base(projectPath) != "Package.swift" {
		return Config{}, fmt.Errorf("invalid project path: should be an .xcodeproj/.xcworkspace or Package.swift file (actual: %s)", projectPath)
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
		return Config{}, errors.New("the 'Relaunch Tests for Each Repetition' (relaunch_tests_for_each_repetition) cannot be used if 'Test Repetition Mode' (test_repetition_mode) is 'none'")
	}

	additionalOptions, err := shellquote.Split(input.XcodebuildOptions)
	if err != nil {
		return Config{}, fmt.Errorf("provided 'Additional options for the xcodebuild command' (xcodebuild_options) (%s) are not valid CLI parameters: %w", input.XcodebuildOptions, err)
	}

	additionalLogFormatterOptions, err := s.parseAdditionalLogFormatterOptions(input.LogFormatter, input.XcprettyOptions, input.LogFormatterOptions)
	if err != nil {
		return Config{}, nil
	}

	if strings.TrimSpace(input.XCConfigContent) == "" {
		input.XCConfigContent = ""
	}
	if sliceutil.IsStringInSlice("-xcconfig", additionalOptions) &&
		input.XCConfigContent != "" {
		return Config{}, fmt.Errorf("`-xcconfig` option found in 'Additional options for the xcodebuild command' (xcodebuild_options), please clear 'Build settings (xcconfig)' (`xcconfig_content`) input as only one can be set")
	}

	return s.utils.CreateConfig(input, projectPath, int(xcodebuildVersion.MajorVersion), sim, additionalOptions, additionalLogFormatterOptions), nil
}

// InstallDeps ...
func (s XcodeTestRunner) InstallDeps() error {
	if s.logFormatterInstaller == nil {
		return nil
	}

	logFormatterVersion, err := s.logFormatterInstaller.Install()
	if err != nil {
		return fmt.Errorf("installing log formatter failed: %w", err)
	}
	s.logger.Printf("- log formatter version: %s", logFormatterVersion.String())
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
	if cfg.CacheLevel == "swift_packages" {
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
		diagnosticsName := filepath.Base(result.SimulatorDiagnosticsPath)

		if err := s.outputExporter.ExportSimulatorDiagnostics(result.DeployDir, result.SimulatorDiagnosticsPath, diagnosticsName); err != nil {
			return fmt.Errorf("failed to export simulator diagnostics: %w", err)
		}
	}

	return nil
}

func (s XcodeTestConfigParser) parseAdditionalLogFormatterOptions(logFormatter, xcprettyOpts, logFormatterOpts string) ([]string, error) {
	opts := logFormatterOpts
	if logFormatter == XcprettyTool && strings.TrimSpace(xcprettyOpts) != "" {
		opts = xcprettyOpts
	}

	switch logFormatter {
	case XcodebuildTool:
		return []string{}, nil
	case XcprettyTool:
		opts := logFormatterOpts
		if strings.TrimSpace(xcprettyOpts) != "" {
			opts = xcprettyOpts
		}

		parsedOpts, err := shellquote.Split(opts)
		if err != nil {
			return []string{}, fmt.Errorf("provided 'Additional options for the xcpretty command' (xcpretty_options) (%s) are not valid CLI parameters: %w", opts, err)
		}

		return parsedOpts, nil
	case XcbeautifyTool:
		parsedOpts, err := shellquote.Split(opts)
		if err != nil {
			return []string{}, fmt.Errorf("provided 'Additional options for the xcbeautify command' (log_formatter_options) (%s) are not valid CLI parameters: %w", opts, err)
		}

		return parsedOpts, nil
	}

	return []string{}, nil
}

func (s XcodeTestConfigParser) validateXcodeVersion(input *Input, xcodeMajorVersion int) error {
	if xcodeMajorVersion < minSupportedXcodeMajorVersion {
		return fmt.Errorf("invalid Xcode major version (%d), should not be less then min supported: %d", xcodeMajorVersion, minSupportedXcodeMajorVersion)
	}

	if input.TestRepetitionMode != xcodebuild.TestRepetitionNone && xcodeMajorVersion < 13 {
		return errors.New("Test Repetition Mode (test_repetition_mode) is not available below Xcode 13")
	}

	if input.RetryTestsOnFailure && xcodeMajorVersion > 12 {
		return errors.New("Should retry tests on failure? (should_retry_test_on_fail) is not available above Xcode 12; use test_repetition_mode=retry_on_failure instead")
	}

	return nil
}

func (s XcodeTestConfigParser) getSimulatorForDestination(destinationSpecifier string) (destination.Device, error) {
	simulatorDestination, err := destination.NewSimulator(destinationSpecifier)
	if err != nil {
		return destination.Device{}, fmt.Errorf("invalid destination specifier (%s): %w", destinationSpecifier, err)
	}
	if simulatorDestination == nil {
		return destination.Device{}, fmt.Errorf("inconsistent state, destination should not be nil")
	}

	device, err := s.deviceFinder.FindDevice(*simulatorDestination)
	if err != nil {
		return destination.Device{}, fmt.Errorf("simulator UDID lookup failed: %w", err)
	}

	s.logger.Infof("Simulator info")
	s.logger.Printf("* simulator_name: %s, version: %s, UDID: %s, status: %s", device.Name, device.OS, device.ID, device.Status)

	return device, nil
}

func (s XcodeTestRunner) prepareSimulator(enableSimulatorVerboseLog bool, simulatorID string, launchSimulator bool) error {
	err := s.simulatorManager.ResetLaunchServices()
	if err != nil {
		s.logger.Warnf("Failed to apply simulator boot workaround: %s", err)
	}

	// Boot simulator
	if enableSimulatorVerboseLog {
		s.logger.Infof("Enabling Simulator verbose log for better diagnostics")
		// Boot the simulator now, so verbose logging can be enabled, and it is kept booted after running tests.
		if err := s.simulatorManager.Boot(simulatorID); err != nil {
			return fmt.Errorf("%v", err)
		}
		if err := s.simulatorManager.EnableVerboseLog(simulatorID); err != nil {
			return fmt.Errorf("%v", err)
		}

		s.logger.Println()
	}

	if launchSimulator {
		s.logger.Infof("Booting simulator (%s)...", simulatorID)

		if err := s.simulatorManager.LaunchWithGUI(simulatorID); err != nil {
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

	swiftPackagesPath, err := s.cache.SwiftPackagesPath(cfg.ProjectPath)
	if err != nil {
		return result, -1, fmt.Errorf("failed to get Swift Packages path: %w", err)
	}

	testParams := s.utils.CreateTestParams(cfg, xcresultPath, swiftPackagesPath)

	testLog, exitCode, testErr := s.xcodebuild.RunTest(testParams)
	result.XcresultPath = xcresultPath
	result.XcodebuildTestLog = testLog

	if testErr != nil || cfg.LogFormatter == XcodebuildTool {
		s.utils.PrintLastLinesOfXcodebuildTestLog(testLog, testErr == nil)
	}

	return result, exitCode, testErr
}

func (s XcodeTestRunner) teardownSimulator(simulatorID string, simulatorDebug exportCondition, isSimulatorBooted bool, testErr error) string {
	var simulatorDiagnosticsPath string

	if simulatorDebug == always || (simulatorDebug == onFailure && testErr != nil) {
		s.logger.Println()
		s.logger.Infof("Collecting Simulator diagnostics")

		diagnosticsPath, err := s.simulatorManager.CollectDiagnostics()
		if err != nil {
			s.logger.Warnf(err.Error())
		} else {
			s.logger.Donef("Simulator diagnostics are available as an artifact (%s)", diagnosticsPath)
			simulatorDiagnosticsPath = diagnosticsPath
		}
	}

	// Shut down the simulator if it was started by the step for diagnostic logs.
	if !isSimulatorBooted && simulatorDebug != never {
		if err := s.simulatorManager.Shutdown(simulatorID); err != nil {
			s.logger.Warnf(err.Error())
		}
	}

	return simulatorDiagnosticsPath
}
