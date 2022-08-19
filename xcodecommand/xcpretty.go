package xcodecommand

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/bitrise-io/go-steputils/v2/ruby"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/hashicorp/go-version"
)

type xcprettyDependencyManager struct {
	logger             log.Logger
	commandFactory     command.Factory
	rubyEnv            ruby.Environment
	rubyCommandFactory ruby.CommandFactory
}

type xcprettyCommandRunner struct {
	logger         log.Logger
	commandFactory command.Factory
}

func NewXcprettyDependencyManager(logger log.Logger, commandFactory command.Factory, rubyCommandFactory ruby.CommandFactory, rubyEnv ruby.Environment) DependencyInstaller {
	return &xcprettyDependencyManager{
		logger:             logger,
		commandFactory:     commandFactory,
		rubyEnv:            rubyEnv,
		rubyCommandFactory: rubyCommandFactory,
	}
}

func NewXcprettyCommandRunner(logger log.Logger, commandFactory command.Factory) Runner {
	return &xcprettyCommandRunner{
		logger:         logger,
		commandFactory: commandFactory,
	}
}

func (c *xcprettyDependencyManager) Install() (*version.Version, error) {
	c.logger.Println()
	c.logger.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := c.isDepInstalled()
	if err != nil {
		return nil, err
	} else if !installed {
		c.logger.Warnf(`xcpretty is not installed`)
		fmt.Println()
		c.logger.Printf("Installing xcpretty")

		cmdModelSlice, err := c.installDep()
		if err != nil {
			return nil, fmt.Errorf("failed to create xcpretty install commands: %w", err)
		}

		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to run xcpretty install command (%s): %w", cmd.PrintableCommandArgs(), err)
			}
		}
	}

	xcprettyVersion, err := c.depVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get xcpretty version: %w", err)
	}

	return xcprettyVersion, nil
}

func (c *xcprettyDependencyManager) isDepInstalled() (bool, error) {
	return c.rubyEnv.IsGemInstalled("xcpretty", "")
}

func (c *xcprettyDependencyManager) installDep() ([]command.Command, error) {
	cmds := c.rubyCommandFactory.CreateGemInstall("xcpretty", "", false, false, nil)
	return cmds, nil
}

func (c *xcprettyDependencyManager) depVersion() (*version.Version, error) {
	cmd := c.commandFactory.Create("xcpretty", []string{"--version"}, nil)

	versionOut, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}

	return version.NewVersion(versionOut)
}

func (c *xcprettyCommandRunner) Run(workDir string, xcodebuildArgs []string, xcprettyArgs []string) (Output, error) {
	var (
		buildOutBuffer         bytes.Buffer
		pipeReader, pipeWriter = io.Pipe()
		buildOutWriter         = io.MultiWriter(&buildOutBuffer, pipeWriter)
		prettyOutWriter        = os.Stdout
	)

	buildCmd := c.commandFactory.Create("xcodebuild", xcodebuildArgs, &command.Opts{
		Stdout: buildOutWriter,
		Stderr: buildOutWriter,
		Env:    xcodeCommandEnvs,
		Dir:    workDir,
	})

	prettyCmd := c.commandFactory.Create("xcpretty", xcprettyArgs, &command.Opts{
		Stdin:  pipeReader,
		Stdout: prettyOutWriter,
		Stderr: prettyOutWriter,
	})

	c.logger.Println()
	c.logger.TInfof("$ set -o pipefail && %s | %v", buildCmd.PrintableCommandArgs(), prettyCmd.PrintableCommandArgs())

	if err := buildCmd.Start(); err != nil {
		return Output{
			RawOut:           buildOutBuffer.Bytes(),
			DidWriteToStdOut: false,
			ExitCode:         1,
		}, err
	}
	if err := prettyCmd.Start(); err != nil {
		return Output{
			RawOut:           buildOutBuffer.Bytes(),
			DidWriteToStdOut: false,
			ExitCode:         1,
		}, err
	}

	defer func() {
		if err := pipeWriter.Close(); err != nil {
			c.logger.Warnf("Failed to close xcodebuild-xcpretty pipe: %s", err)
		}

		if err := prettyCmd.Wait(); err != nil {
			c.logger.Warnf("xcpretty command failed: %s", err)
		}
	}()

	if err := buildCmd.Wait(); err != nil {
		var exerr *exec.ExitError
		if errors.As(err, &exerr) {
			return Output{
				RawOut:           buildOutBuffer.Bytes(),
				DidWriteToStdOut: false,
				ExitCode:         exerr.ExitCode(),
			}, err
		}

		return Output{
			RawOut:           buildOutBuffer.Bytes(),
			DidWriteToStdOut: false,
			ExitCode:         1,
		}, err
	}

	return Output{
		RawOut:           buildOutBuffer.Bytes(),
		DidWriteToStdOut: false,
		ExitCode:         0,
	}, nil
}
