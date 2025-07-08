package step

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/progress"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-io/go-xcode/v2/simulator"
	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
	"github.com/bitrise-io/go-xcode/v2/xcodecommand"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/kballard/go-shellquote"
)

const simulatorShutdownState = "Shutdown"

type Input struct {
	ProjectPath string `env:"project_path,required"`
	Scheme      string `env:"scheme,required"`
	Destination string `env:"destination,required"`
	TestPlan    string `env:"test_plan"`

	// Test Repetition
	TestRepetitionMode             string `env:"test_repetition_mode,opt[none,until_failure,retry_on_failure,up_until_maximum_repetitions]"`
	MaximumTestRepetitions         int    `env:"maximum_test_repetitions,required"`
	RelaunchTestsForEachRepetition bool   `env:"relaunch_tests_for_each_repetition,opt[yes,no]"`

	// xcodebuild configuration
	XCConfigContent    string `env:"xcconfig_content"`
	PerformCleanAction bool   `env:"perform_clean_action,opt[yes,no]"`
	XcodebuildOptions  string `env:"xcodebuild_options"`

	// xcodebuild log formatting
	LogFormatter      string `env:"log_formatter,opt[xcbeautify,xcodebuild,xcpretty]"`
	XcprettyOptions   string `env:"xcpretty_options"`
	XcbeautifyOptions string `env:"xcbeautify_options"`

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

// Output tools
const (
	XcbeautifyTool = "xcbeautify"
	XcodebuildTool = "xcodebuild"
	XcprettyTool   = "xcpretty"
)

type Config struct {
	ProjectPath string
	Scheme      string
	TestPlan    string

	Simulator         destination.Device
	IsSimulatorBooted bool

	TestRepetitionMode            string
	MaximumTestRepetitions        int
	RelaunchTestForEachRepetition bool

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
	logger       log.Logger
	inputParser  stepconf.InputParser
	deviceFinder destination.DeviceFinder
	pathModifier pathutil.PathModifier
	utils        Utils
}

func NewXcodeTestConfigParser(inputParser stepconf.InputParser, logger log.Logger, deviceFinder destination.DeviceFinder, pathModifier pathutil.PathModifier, utils Utils) XcodeTestConfigParser {
	return XcodeTestConfigParser{
		logger:       logger,
		inputParser:  inputParser,
		deviceFinder: deviceFinder,
		pathModifier: pathModifier,
		utils:        utils,
	}
}

type XcodeTestRunner struct {
	logger           log.Logger
	commandFactory   command.Factory
	xcodebuild       xcodebuild.Xcodebuild
	simulatorManager simulator.Manager
	cache            cache.SwiftPackageCache
	outputExporter   output.Exporter
	pathModifier     pathutil.PathModifier
	pathProvider     pathutil.PathProvider
	utils            Utils
}

func NewXcodeTestRunner(logger log.Logger, commandFactory command.Factory, xcodebuild xcodebuild.Xcodebuild, simulatorManager simulator.Manager, cache cache.SwiftPackageCache, outputExporter output.Exporter, pathModifier pathutil.PathModifier, pathProvider pathutil.PathProvider, utils Utils) XcodeTestRunner {
	return XcodeTestRunner{
		logger:           logger,
		commandFactory:   commandFactory,
		xcodebuild:       xcodebuild,
		simulatorManager: simulatorManager,
		cache:            cache,
		outputExporter:   outputExporter,
		pathModifier:     pathModifier,
		pathProvider:     pathProvider,
		utils:            utils,
	}
}

func (s XcodeTestConfigParser) ProcessConfig() (Config, error) {
	var input Input
	err := s.inputParser.Parse(&input)
	if err != nil {
		return Config{}, err
	}

	stepconf.Print(input)
	s.logger.EnableDebugLog(input.VerboseLog)

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

	additionalLogFormatterOptions, err := s.parseAdditionalLogFormatterOptions(input.LogFormatter, input.XcprettyOptions, input.XcbeautifyOptions)
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

	return s.utils.CreateConfig(input, projectPath, sim, additionalOptions, additionalLogFormatterOptions), nil
}

func (s XcodeTestRunner) InstallDeps() {
	logFormatterVersion, err := s.xcodebuild.GetXcodeCommadRunner().CheckInstall()
	if err != nil {
		s.logger.Errorf("Selected log formatter is unavailable:: %s", err)
		s.logger.Infof("Switching back to xcodebuild log formatter.")
		s.xcodebuild.SetXcodeCommandRunner(xcodecommand.NewRawCommandRunner(s.logger, s.commandFactory))

		return
	}

	if logFormatterVersion != nil { // raw xcodebuild runner returns nil
		s.logger.Printf("- log formatter version: %s", logFormatterVersion.String())
	}
}

type Result struct {
	Scheme    string
	DeployDir string

	XcresultPath             string
	XcodebuildBuildLog       string
	XcodebuildTestLog        string
	SimulatorDiagnosticsPath string
}

func (s XcodeTestRunner) Run(cfg Config) (Result, error) {
	enableSimulatorVerboseLog := cfg.CollectSimulatorDiagnostics != never
	launchSimulator := !cfg.IsSimulatorBooted && !cfg.HeadlessMode
	if err := s.prepareSimulator(enableSimulatorVerboseLog, cfg.Simulator, launchSimulator); err != nil {
		return Result{}, err
	}

	s.logger.Println()
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

	result.SimulatorDiagnosticsPath = s.teardownSimulator(cfg.Simulator.UDID, cfg.CollectSimulatorDiagnostics, cfg.IsSimulatorBooted, testErr)

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

func (s XcodeTestRunner) Export(result Result, testFailed bool) error {
	// export test run status
	s.outputExporter.ExportTestRunResult(testFailed)

	if result.XcresultPath != "" {
		s.outputExporter.ExportXCResultBundle(result.DeployDir, result.XcresultPath, result.Scheme)
		if err := s.outputExporter.ExportFlakyTestCases(result.XcresultPath, false); err != nil {
			s.logger.Warnf("Failed to export flaky test cases: %s", err)
		}
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

func (s XcodeTestConfigParser) parseAdditionalLogFormatterOptions(logFormatter, xcprettyOpts, xcbeautifyOpts string) ([]string, error) {
	switch logFormatter {
	case XcodebuildTool:
		return []string{}, nil
	case XcprettyTool:
		parsedOpts, err := shellquote.Split(xcprettyOpts)
		if err != nil {
			return []string{}, fmt.Errorf("provided 'Additional options for the xcpretty command' (xcpretty_options) (%s) are not valid CLI parameters: %w", xcprettyOpts, err)
		}

		return parsedOpts, nil
	case XcbeautifyTool:
		parsedOpts, err := shellquote.Split(xcbeautifyOpts)
		if err != nil {
			return []string{}, fmt.Errorf("provided 'Additional options for the xcbeautify command' (xcbeautify_options) (%s) are not valid CLI parameters: %w", xcbeautifyOpts, err)
		}

		return parsedOpts, nil
	default:
		panic(fmt.Sprintf("Unknown log formatter: %s", logFormatter))
	}
}

func (s XcodeTestConfigParser) getSimulatorForDestination(destinationSpecifier string) (destination.Device, error) {
	simulatorDestination, err := destination.NewSimulator(destinationSpecifier)
	if err != nil {
		return destination.Device{}, fmt.Errorf("invalid destination specifier (%s): %w", destinationSpecifier, err)
	}

	device, err := s.deviceFinder.FindDevice(*simulatorDestination)
	if err != nil {
		destinationLabel := fmt.Sprintf("%s %s on %s", simulatorDestination.Platform, simulatorDestination.OS, simulatorDestination.Name)
		return destination.Device{}, fmt.Errorf(
			"no matching device is available for the provided destination (%s): %w",
			colorstring.Cyan(destinationLabel),
			err,
		)
	}

	s.logger.Println()
	s.logger.Infof("Destination simulator:")
	s.logger.Printf("Name: %s", colorstring.Cyan(device.Name))
	if device.Name != device.Type {
		s.logger.Printf("Device type: %s", colorstring.Cyan(device.Type))
	}
	s.logger.Printf("OS: %s %s", colorstring.Cyan(device.Platform), colorstring.Cyan(device.OS))
	s.logger.Printf("UDID: %s", colorstring.Cyan(device.UDID))
	s.logger.Printf("Status: %s", colorstring.Cyan(device.State))

	return device, nil
}

func (s XcodeTestRunner) prepareSimulator(enableSimulatorVerboseLog bool, simulator destination.Device, launchSimulator bool) error {
	err := s.simulatorManager.ResetLaunchServices()
	if err != nil {
		s.logger.Warnf("Failed to apply simulator boot workaround: %s", err)
	}

	// Boot simulator
	if enableSimulatorVerboseLog {
		s.logger.Infof("Enabling Simulator verbose log for better diagnostics")
		// Boot the simulator now, so verbose logging can be enabled, and it is kept booted after running tests.
		if err := s.simulatorManager.Boot(simulator); err != nil {
			return fmt.Errorf("%v", err)
		}
		if err := s.simulatorManager.EnableVerboseLog(simulator.UDID); err != nil {
			return fmt.Errorf("%v", err)
		}

		s.logger.Println()
	}

	if launchSimulator {
		s.logger.Infof("Booting simulator (%s)...", simulator.UDID)

		if err := s.simulatorManager.LaunchWithGUI(simulator.UDID); err != nil {
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
