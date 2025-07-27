package step

import (
	"fmt"

	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/stringutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/destination"
	"github.com/bitrise-steplib/steps-xcode-test/xcodebuild"
)

type Utils interface {
	PrintLastLinesOfXcodebuildTestLog(rawXcodebuildOutput string, isRunSuccess bool)
	CreateConfig(input Input, projectPath string, sim destination.Device, additionalOptions, additionalLogFormatterOptions []string) Config
	CreateTestParams(cfg Config, xcresultPath string) xcodebuild.TestRunParams
}

type utils struct {
	logger log.Logger
}

func NewUtils(logger log.Logger) Utils {
	return &utils{
		logger: logger,
	}
}

func (u utils) PrintLastLinesOfXcodebuildTestLog(rawXcodebuildOutput string, isRunSuccess bool) {
	const lastLines = "\nLast lines of the build log:"
	if !isRunSuccess {
		u.logger.Errorf(lastLines)
	} else {
		u.logger.Infof(lastLines)
	}

	fmt.Println(stringutil.LastNLines(rawXcodebuildOutput, 20))

	if !isRunSuccess {
		u.logger.Warnf("If you can't find the reason of the error in the log, please check the xcodebuild_test.log.")
	}

	u.logger.Infof(colorstring.Magenta(`
The log file is stored in $BITRISE_DEPLOY_DIR, and its full path
is available in the $BITRISE_XCODEBUILD_TEST_LOG_PATH environment variable.

If you have the Deploy to Bitrise.io step (after this step),
that will attach the file to your build as an artifact!`))

}

func (u utils) CreateConfig(input Input,
	projectPath string,
	sim destination.Device,
	additionalOptions, additionalLogFormatterOptions []string) Config {
	return Config{
		ProjectPath: projectPath,
		Scheme:      input.Scheme,
		TestPlan:    input.TestPlan,

		Simulator:         sim,
		IsSimulatorBooted: sim.State != simulatorShutdownState,

		TestRepetitionMode:            input.TestRepetitionMode,
		MaximumTestRepetitions:        input.MaximumTestRepetitions,
		RelaunchTestForEachRepetition: input.RelaunchTestsForEachRepetition,

		XCConfigContent:    input.XCConfigContent,
		PerformCleanAction: input.PerformCleanAction,
		XcodebuildOptions:  additionalOptions,

		LogFormatter:        input.LogFormatter,
		LogFormatterOptions: additionalLogFormatterOptions,

		CollectSimulatorDiagnostics: exportCondition(input.CollectSimulatorDiagnostics),
		HeadlessMode:                input.HeadlessMode,

		DeployDir: input.DeployDir,
	}
}

func (u utils) CreateTestParams(cfg Config, xcresultPath string) xcodebuild.TestRunParams {
	testParams := xcodebuild.TestParams{
		ProjectPath:                    cfg.ProjectPath,
		Scheme:                         cfg.Scheme,
		Destination:                    cfg.Simulator.XcodebuildDestination(),
		TestPlan:                       cfg.TestPlan,
		TestOutputDir:                  xcresultPath,
		TestRepetitionMode:             cfg.TestRepetitionMode,
		MaximumTestRepetitions:         cfg.MaximumTestRepetitions,
		RelaunchTestsForEachRepetition: cfg.RelaunchTestForEachRepetition,
		XCConfigContent:                cfg.XCConfigContent,
		PerformCleanAction:             cfg.PerformCleanAction,
		AdditionalOptions:              cfg.XcodebuildOptions,
	}

	return xcodebuild.TestRunParams{
		TestParams:                         testParams,
		LogFormatterOptions:                cfg.LogFormatterOptions,
		RetryOnTestRunnerError:             true,
	}
}
