package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
)

func simulatorBoot(id string) error {
	bootSimulatorCommand := command.NewWithStandardOuts("xcrun", "simctl", "boot", id)

	log.Donef("$ %s", bootSimulatorCommand.PrintableCommandArgs())
	exitCode, err := bootSimulatorCommand.RunAndReturnExitCode()
	if err != nil {
		if errorutil.IsExitStatusError(err) {
			if exitCode == 149 { // Simulator already booted
				return nil
			}
			log.Warnf("Failed to boot Simulator, command exited with code %d", exitCode)
			return nil
		}
		return fmt.Errorf("failed to boot Simulator, command execution failed: %v", err)
	}

	return nil
}

func simulatorShutdown(id string) error {
	bootSimulatorCommand := command.NewWithStandardOuts("xcrun", "simctl", "shutdown", id)

	log.Donef("$ %s", bootSimulatorCommand.PrintableCommandArgs())
	exitCode, err := bootSimulatorCommand.RunAndReturnExitCode()
	if err != nil {
		if errorutil.IsExitStatusError(err) {
			if exitCode == 149 { // Simulator already shut down
				return nil
			}
			log.Warnf("Failed to shutdown Simulator, command exited with code %d", exitCode)
			return nil
		}
		return fmt.Errorf("failed to shutdown Simulator, command execution failed: %v", err)
	}

	return nil
}

// Simulator needs to be booted to enable verbose log
func simulatorEnableVerboseLog(id string) error {
	simulatorVerboseCommand := command.NewWithStandardOuts("xcrun", "simctl", "logverbose", id, "enable")

	log.Donef("$ %s", simulatorVerboseCommand.PrintableCommandArgs())
	if err := simulatorVerboseCommand.Run(); err != nil {
		if errorutil.IsExitStatusError(err) {
			log.Warnf("Failed to enable Simulator verbose logging, command exited with code %d", err)
			return nil
		}

		return fmt.Errorf("failed to enable Simulator verbose logging, command execution failed: %v", err)
	}

	return nil
}

func simulatorDiagnosticsName() (string, error) {
	timestamp, err := time.Now().MarshalText()
	if err != nil {
		return "", fmt.Errorf("failed to collect Simulator diagnostics, failed to marshal timestamp: %v", err)
	}

	return fmt.Sprintf("simctl_diagnose_%s.zip", strings.ReplaceAll(string(timestamp), ":", "-")), nil
}

func simulatorCollectDiagnostics() (string, error) {
	diagnosticsName, err := simulatorDiagnosticsName()
	if err != nil {
		return "", err
	}
	diagnosticsOutDir, err := ioutil.TempDir("", diagnosticsName)
	if err != nil {
		return "", fmt.Errorf("failed to collect Simulator diagnostics, could not create temporary directory: %v", err)
	}

	simulatorDiagnosticsCommand := command.NewWithStandardOuts("xcrun", "simctl", "diagnose", "-b", "--no-archive", fmt.Sprintf("--output=%s", diagnosticsOutDir))
	simulatorDiagnosticsCommand.SetStdin(bytes.NewReader([]byte("\n")))

	log.Donef("$ %s", simulatorDiagnosticsCommand.PrintableCommandArgs())
	if err := simulatorDiagnosticsCommand.Run(); err != nil {
		if errorutil.IsExitStatusError(err) {
			return "", fmt.Errorf("failed to collect Simulator diagnostics: %v", err)

		}
		return "", fmt.Errorf("failed to collect Simulator diagnostics, command execution failed: %v", err)
	}

	return diagnosticsOutDir, nil
}
