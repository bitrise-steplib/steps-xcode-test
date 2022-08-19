package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-steputils/v2/ruby"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/stepenv"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-io/go-xcode/v2/simulator"
	"github.com/bitrise-io/go-xcode/v2/xcconfig"
	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/step"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/bitrise-steplib/steps-xcode-test/xcodecommand"
	"github.com/bitrise-steplib/steps-xcode-test/xcodeversion"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := log.NewLogger()
	configParser := createConfigParser(logger)
	config, err := configParser.ProcessConfig()
	if err != nil {
		logger.Errorf("Process config: %s", err)
		return 1
	}

	xcodeTestRunner, err := createStep(logger, config.LogFormatter)
	if err != nil {
		logger.Errorf("Process conifg: %s", err)
	}

	if err := xcodeTestRunner.InstallDeps(config.LogFormatter == xcodebuild.XcprettyTool); err != nil {
		logger.Warnf("Install dependencies: %s", err)
		logger.Printf("Switching to xcodebuild for output tool")
		config.LogFormatter = xcodebuild.XcodebuildTool
	}

	res, runErr := xcodeTestRunner.Run(config)
	exportErr := xcodeTestRunner.Export(res, runErr != nil)

	if runErr != nil {
		logger.Errorf("Run: %s", runErr)
		return 1
	}

	if exportErr != nil {
		logger.Errorf("Export outputs: %s", err)
		return 1
	}

	return 0
}

func createConfigParser(logger log.Logger) step.XcodeTestConfigParser {
	envRepository := env.NewRepository()
	commandFactory := command.NewFactory(envRepository)
	inputParser := stepconf.NewInputParser(envRepository)
	xcodeVersionReader := xcodeversion.NewXcodeVersionReader()
	pathModifier := pathutil.NewPathModifier()
	deviceFinder := destination.NewDeviceFinder(logger, commandFactory)
	utils := step.NewUtils(logger)

	return step.NewXcodeTestConfigParser(inputParser, logger, xcodeVersionReader, deviceFinder, pathModifier, utils)
}

func createStep(logger log.Logger, logFormatter string) (step.XcodeTestRunner, error) {
	envRepository := env.NewRepository()
	commandFactory := command.NewFactory(envRepository)
	pathChecker := pathutil.NewPathChecker()
	pathProvider := pathutil.NewPathProvider()
	pathModifier := pathutil.NewPathModifier()
	fileManager := fileutil.NewFileManager()
	xcconfigWriter := xcconfig.NewWriter(pathProvider, fileManager, pathChecker, pathModifier)
	xcodeCommandRunner := xcodecommand.NewRawCommandRunner(logger, commandFactory)
	xcodebuilder := xcodebuild.NewXcodebuild(logger, pathChecker, fileManager, xcconfigWriter, xcodeCommandRunner)
	simulatorManager := simulator.NewManager(logger, commandFactory)
	swiftCache := cache.NewSwiftPackageCache()
	testAddonExporter := testaddon.NewExporter(testaddon.NewTestAddon(logger))
	stepenvRepository := stepenv.NewRepository(envRepository)
	outputExporter := output.NewExporter(stepenvRepository, logger, testAddonExporter)
	utils := step.NewUtils(logger)

	var xcodeRunnerDepInstaller xcodecommand.DependencyInstaller
	if logFormatter == xcodebuild.XcprettyTool {
		commandLocator := env.NewCommandLocator()
		rubyComamndFactory, err := ruby.NewCommandFactory(commandFactory, commandLocator)
		if err != nil {
			return step.XcodeTestRunner{}, fmt.Errorf("failed to install xcpretty: %s", err)
		}
		rubyEnv := ruby.NewEnvironment(rubyComamndFactory, commandLocator, logger)
		xcodeRunnerDepInstaller = xcodecommand.NewXcprettyDependencyManager(logger, commandFactory, rubyComamndFactory, rubyEnv)
	}

	return step.NewXcodeTestRunner(logger, xcodeRunnerDepInstaller, xcodebuilder, simulatorManager, swiftCache, outputExporter, pathModifier, pathProvider, utils), nil
}
