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

type xcbeautifyRunner struct {
	logger         log.Logger
	commandFactory command.Factory
}

func NewXcbeautifyRunner(logger log.Logger, commandFactory command.Factory) Runner {
	return &xcbeautifyRunner{
		logger:         logger,
		commandFactory: commandFactory,
	}
}

// set -o pipefail && xcodebuild [flags] | xcbeautify
// NSUnbufferedIO=YES xcodebuild [flags] 2>&1 | xcbeautify
func (c xcbeautifyRunner) Run(workDir string, xcodebuildArgs []string, _ []string) (Output, error) {
	var (
		buildOutBuffer         bytes.Buffer
		pipeReader, pipeWriter = io.Pipe()
		buildOutWriter         = io.MultiWriter(&buildOutBuffer, pipeWriter)
	)

	// For parallel and concurrent destination testing, it helps to use unbuffered I/O for stdout and to redirect stderr to stdout.
	// NSUnbufferedIO=YES xcodebuild [args] 2>&1 | xcbeautify
	buildCmd := c.commandFactory.Create("xcodebuild", xcodebuildArgs, &command.Opts{
		Stdout: buildOutWriter,
		Stderr: buildOutWriter,
		Env:    xcodeCommandEnvs,
		Dir:    workDir,
	})

	beautifyCmd := c.commandFactory.Create("xcbeautify", []string{}, &command.Opts{
		Stdin:  pipeReader,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})

	c.logger.TPrintf("$ set -o pipefail && %s | %v", buildCmd.PrintableCommandArgs(), beautifyCmd.PrintableCommandArgs())

	if err := buildCmd.Start(); err != nil {
		return Output{
			RawOut:   buildOutBuffer.Bytes(),
			ExitCode: 1,
		}, err
	}
	if err := beautifyCmd.Start(); err != nil {
		return Output{
			RawOut:   buildOutBuffer.Bytes(),
			ExitCode: 1,
		}, err
	}

	defer func() {
		if err := pipeWriter.Close(); err != nil {
			c.logger.Warnf("Failed to close xcodebuild-xcbeautify pipe: %s", err)
		}

		if err := beautifyCmd.Wait(); err != nil {
			c.logger.Warnf("xcbeautify command failed: %s", err)
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
