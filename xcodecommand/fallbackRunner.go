package xcodecommand

import (
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
	version "github.com/hashicorp/go-version"
)

type xcodecommandRunner struct {
	installer DependencyInstaller
	runner    Runner
}

type FallbackRunner struct {
	runner         xcodecommandRunner
	fallbackRunner xcodecommandRunner
	logger         log.Logger
}

func NewFallbackRunner(runner Runner, installer DependencyInstaller, logger log.Logger, commandFactory command.Factory) *FallbackRunner {
	return &FallbackRunner{
		runner: xcodecommandRunner{
			runner:    runner,
			installer: installer,
		},
		fallbackRunner: xcodecommandRunner{
			runner:    NewRawCommandRunner(logger, commandFactory),
			installer: nil,
		},
		logger: logger,
	}
}

func (sel *FallbackRunner) CheckInstall() (*version.Version, error) {
	if sel.runner.installer == nil {
		return nil, nil
	}

	ver, err := sel.runner.installer.CheckInstall()
	if err == nil {
		return ver, nil
	}

	sel.logger.Errorf("Checking log formatter failed: %s", err)
	sel.logger.Infof("Falling back to xcodebuild log formatter")
	sel.runner = sel.fallbackRunner

	if sel.runner.installer == nil {
		return nil, nil
	}
	return sel.runner.installer.CheckInstall()
}

func (sel *FallbackRunner) Run(workDir string, xcodebuildArgs []string, xcbeautifyArgs []string) (Output, error) {
	return sel.runner.runner.Run(workDir, xcodebuildArgs, xcbeautifyArgs)
}
