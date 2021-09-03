package main

import (
	"os"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-steputils/stepenv"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-xcode-test/cache"
	"github.com/bitrise-steplib/steps-xcode-test/output"
	"github.com/bitrise-steplib/steps-xcode-test/simulator"
	"github.com/bitrise-steplib/steps-xcode-test/step"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
	"github.com/bitrise-steplib/steps-xcode-test/testartifact"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
	"github.com/bitrise-steplib/steps-xcode-test/xcpretty"
)

func createStep(logger log.Logger) step.XcodeTestRunner {
	envRepository := env.NewRepository()
	inputParser := stepconf.NewInputParser(envRepository)
	xcprettyInstaller := xcpretty.NewInstaller()
	commandFactory := command.NewFactory(envRepository)
	pathChecker := pathutil.NewPathChecker()
	fileRemover := fileutil.NewFileRemover()
	xcodebuilder := xcodebuild.New(logger, commandFactory, pathChecker, fileRemover)
	sim := simulator.NewSimulator()
	c := cache.NewSwiftPackageCache()
	testAddonExporter := testaddon.NewExporter()
	testArtifactExporter := testartifact.NewExporter()
	stepenvRepository := stepenv.NewRepository(envRepository)
	outputExporter := output.NewExporter(stepenvRepository, logger, testAddonExporter, testArtifactExporter)
	pathModifier := pathutil.NewPathModifier()

	return step.NewXcodeTestRunner(inputParser, logger, xcprettyInstaller, xcodebuilder, sim, c, outputExporter, pathModifier)
}

func run() int {
	logger := log.NewLogger()
	xcodeTestRunner := createStep(logger)
	config, err := xcodeTestRunner.ProcessConfig()
	if err != nil {
		logger.Errorf(err.Error())
		return 1

	}

	if err := xcodeTestRunner.InstallDeps(config.OutputTool == xcodebuild.XcprettyTool); err != nil {
		logger.Warnf("Failed to install deps: %s", err)
		logger.Printf("Switching to xcodebuild for output tool")
		config.OutputTool = xcodebuild.XcodebuildTool
	}

	res, runErr := xcodeTestRunner.Run(config)

	opts := step.ExportOpts{
		TestFailed: runErr != nil,

		Scheme:       config.Scheme,
		DeployDir:    config.DeployDir,
		XcresultPath: res.XcresultPath,

		XcodebuildBuildLog: res.XcodebuildBuildLog,
		XcodebuildTestLog:  res.XcodebuildTestLog,

		SimulatorDiagnosticsPath: res.SimulatorDiagnosticsPath,
		ExportUITestArtifacts:    config.ExportUITestArtifacts,
	}
	exportErr := xcodeTestRunner.Export(opts)

	if runErr != nil {
		logger.Errorf(runErr.Error())
		return 1
	}

	if exportErr != nil {
		logger.Errorf(exportErr.Error())
		return 1
	}

	return 0
}

func main() {
	os.Exit(run())
}
