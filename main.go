package main

import (
	"os"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/stepenv"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-xcode/v2/xcconfig"
	cache "github.com/bitrise-io/go-xcode/v2/xcodecache"
	goxcpretty "github.com/bitrise-io/go-xcode/v2/xcpretty"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/simulator"
	"github.com/bitrise-steplib/steps-xcode-test/step"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/bitrise-steplib/steps-xcode-test/xcpretty"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := log.NewLogger()
	xcodeTestRunner := createStep(logger)
	config, err := xcodeTestRunner.ProcessConfig()
	if err != nil {
		logger.Errorf("Process config: %s", err)
		return 1

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

func createStep(logger log.Logger) step.XcodeTestRunner {
	envRepository := env.NewRepository()
	inputParser := stepconf.NewInputParser(envRepository)
	xcprettyInstaller := xcpretty.NewInstaller(goxcpretty.NewXcpretty(logger), logger)
	commandFactory := command.NewFactory(envRepository)
	pathChecker := pathutil.NewPathChecker()
	pathProvider := pathutil.NewPathProvider()
	pathModifier := pathutil.NewPathModifier()
	fileManager := fileutil.NewFileManager()
	xcconfigWriter := xcconfig.NewWriter(pathProvider, fileManager, pathChecker, pathModifier)
	xcodebuilder := xcodebuild.NewXcodebuild(logger, commandFactory, pathChecker, fileManager, xcconfigWriter)
	simulatorManager := simulator.NewManager(commandFactory, logger)
	swiftCache := cache.NewSwiftPackageCache()
	testAddonExporter := testaddon.NewExporter(testaddon.NewTestAddon(logger))
	stepenvRepository := stepenv.NewRepository(envRepository)
	outputExporter := output.NewExporter(stepenvRepository, logger, testAddonExporter)
	utils := step.NewUtils(logger)

	return step.NewXcodeTestRunner(inputParser, logger, xcprettyInstaller, xcodebuilder, simulatorManager, swiftCache, outputExporter, pathModifier, pathProvider, utils)
}
