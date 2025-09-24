package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-steputils/v2/ruby"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/stepenv"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/errorutil"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-io/go-xcode/v2/simulator"
	"github.com/bitrise-io/go-xcode/v2/xcconfig"
	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
	"github.com/bitrise-io/go-xcode/v2/xcodecommand"
	"github.com/bitrise-io/go-xcode/v2/xcodeversion"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/step"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := log.NewLogger()
	configParser := createConfigParser(logger)
	config, err := configParser.ProcessConfig()
	if err != nil {
		logger.Errorf(errorutil.FormattedError(fmt.Errorf("Failed to process Step inputs: %w", err)))
		return 1
	}

	xcodeTestRunner, err := createStep(logger, config.LogFormatter)
	if err != nil {
		logger.Errorf(errorutil.FormattedError(fmt.Errorf("Failed to process Step inputs: %w", err)))
		return 1
	}

	xcodeTestRunner.InstallDeps()

	res, runErr := xcodeTestRunner.Run(config)
	exportErr := xcodeTestRunner.Export(res, runErr != nil)

	if runErr != nil {
		logger.Errorf(errorutil.FormattedError(fmt.Errorf("Failed to execute Step: %w", runErr)))
		return 1
	}

	if exportErr != nil {
		logger.Errorf(errorutil.FormattedError(fmt.Errorf("Failed to export Step outputs: %w", exportErr)))
		return 1
	}

	return 0
}

func createConfigParser(logger log.Logger) step.XcodeTestConfigParser {
	envRepository := env.NewRepository()
	commandFactory := command.NewFactory(envRepository)
	inputParser := stepconf.NewInputParser(envRepository)
	xcodeVersionProvider := xcodeversion.NewXcodeVersionProvider(commandFactory)
	xcodeVersion, err := xcodeVersionProvider.GetVersion()
	if err != nil { // Not a fatal error, continuing with empty version
		logger.Errorf("failed to read Xcode version: %w", err)
	}
	pathModifier := pathutil.NewPathModifier()
	deviceFinder := destination.NewDeviceFinder(logger, commandFactory, xcodeVersion)
	utils := step.NewUtils(logger)

	return step.NewXcodeTestConfigParser(inputParser, logger, deviceFinder, pathModifier, utils)
}

func createStep(logger log.Logger, logFormatter string) (step.XcodeTestRunner, error) {
	envRepository := env.NewRepository()
	commandFactory := command.NewFactory(envRepository)
	pathChecker := pathutil.NewPathChecker()
	pathProvider := pathutil.NewPathProvider()
	pathModifier := pathutil.NewPathModifier()
	fileManager := fileutil.NewFileManager()
	xcconfigWriter := xcconfig.NewWriter(pathProvider, fileManager, pathChecker, pathModifier)
	simulatorManager := simulator.NewManager(logger, commandFactory)
	swiftCache := cache.NewSwiftPackageCache()
	outputExporter := export.NewExporter(commandFactory)
	testAddonExporter := testaddon.NewExporter(testaddon.NewTestAddon(logger))
	stepenvRepository := stepenv.NewRepository(envRepository)
	exporter := output.NewExporter(stepenvRepository, logger, outputExporter, testAddonExporter)
	utils := step.NewUtils(logger)

	xcodeCommandRunner := xcodecommand.Runner(nil)
	switch logFormatter {
	case step.XcodebuildTool:
		xcodeCommandRunner = xcodecommand.NewRawCommandRunner(logger, commandFactory)
	case step.XcbeautifyTool:
		xcodeCommandRunner = xcodecommand.NewXcbeautifyRunner(logger, commandFactory)
	case step.XcprettyTool:
		commandLocator := env.NewCommandLocator()
		rubyComamndFactory, err := ruby.NewCommandFactory(commandFactory, commandLocator)
		if err != nil {
			return step.XcodeTestRunner{}, fmt.Errorf("failed to install xcpretty: %s", err)
		}
		rubyEnv := ruby.NewEnvironment(rubyComamndFactory, commandLocator, logger)

		xcodeCommandRunner = xcodecommand.NewXcprettyCommandRunner(logger, commandFactory, pathChecker, fileManager, rubyComamndFactory, rubyEnv)
	default:
		panic(fmt.Sprintf("Unknown log formatter: %s", logFormatter))
	}

	xcodebuilder := xcodebuild.NewXcodebuild(logger, fileManager, xcconfigWriter, xcodeCommandRunner)

	return step.NewXcodeTestRunner(logger, commandFactory, xcodebuilder, simulatorManager, swiftCache, exporter, pathModifier, pathProvider, utils), nil
}
