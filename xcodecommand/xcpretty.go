package xcodecommand

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
)

type xcprettyCommandRunner struct {
	logger         log.Logger
	commandFactory command.Factory
}

func NewXcprettyCommandRunner(logger log.Logger, commandFactory command.Factory) Runner {
	return &xcprettyCommandRunner{
		logger:         logger,
		commandFactory: commandFactory,
	}
}

func (c *xcprettyCommandRunner) Run(workDir string, xcodebuildArgs []string, xcprettyArgs []string) (Output, error) {
	var (
		buildOutBuffer         bytes.Buffer
		pipeReader, pipeWriter = io.Pipe()
		buildOutWriter         = io.MultiWriter(&buildOutBuffer, pipeWriter)
		prettyOutWriter        = os.Stdout
	)

	defer func() {
		if err := pipeWriter.Close(); err != nil {
			c.logger.Warnf("Failed to close xcodebuild-xcpretty pipe: %s", err)
		}
	}()

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

	c.logger.TPrintf("$ set -o pipefail && %s | %v", buildCmd.PrintableCommandArgs(), prettyCmd.PrintableCommandArgs())

	if err := buildCmd.Start(); err != nil {
		return Output{
			RawOut:   buildOutBuffer.Bytes(),
			ExitCode: 1,
		}, err
	}
	if err := prettyCmd.Start(); err != nil {
		return Output{
			RawOut:   buildOutBuffer.Bytes(),
			ExitCode: 1,
		}, err
	}

	defer func() {
		if err := prettyCmd.Wait(); err != nil {
			c.logger.Warnf("xcpretty command failed: %s", err)
		}
	}()

	if err := buildCmd.Wait(); err != nil {
		var exerr *exec.ExitError
		if errors.As(err, &exerr) {
			return Output{
				RawOut:   buildOutBuffer.Bytes(),
				ExitCode: exerr.ExitCode(),
			}, err
		}

		return Output{
			RawOut:   buildOutBuffer.Bytes(),
			ExitCode: 1,
		}, err
	}

	return Output{
		RawOut:   buildOutBuffer.Bytes(),
		ExitCode: 0,
	}, nil
}
