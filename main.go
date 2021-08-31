package main

import (
	"os"

	"github.com/bitrise-steplib/steps-xcode-test/output"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-steputils/stepenv"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-steplib/steps-xcode-test/simulator"
	"github.com/bitrise-steplib/steps-xcode-test/step"
	"github.com/bitrise-steplib/steps-xcode-test/testaddon"
	"github.com/bitrise-steplib/steps-xcode-test/testartifact"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
)

func run() int {
	logger := log.NewLogger()
	envRepository := env.NewRepository()
	inputParser := stepconf.NewInputParser(envRepository)
	stepenvRepository := stepenv.NewRepository(envRepository)
	xcodebuilder := xcodebuild.New()
	sim := simulator.New()
	testAddonExporter := testaddon.NewExporter()
	testArtifactExporter := testartifact.NewExporter()
	outputExporter := output.NewExporter(stepenvRepository, logger, testAddonExporter, testArtifactExporter)

	xcodeTestRunner := step.NewXcodeTestRunner(inputParser, logger, xcodebuilder, sim, outputExporter)

	config, err := xcodeTestRunner.ProcessConfig()
	if err != nil {
		logger.Errorf(err.Error())
		return 1

	}

	if err := xcodeTestRunner.InstallDeps(config.OutputTool == step.XcprettyTool); err != nil {
		logger.Warnf("Failed to install deps: %s", err)
		config.OutputTool = step.XcodebuildTool
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
