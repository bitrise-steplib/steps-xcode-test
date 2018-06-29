package main

import (
	"fmt"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/command/rubycommand"
	"github.com/bitrise-io/go-utils/log"
	version "github.com/hashicorp/go-version"
)

// IsToolInstalled ...
func IsToolInstalled(name, version string) (bool, error) {
	return rubycommand.IsGemInstalled(name, version)
}

// IsXcprettyInstalled ...
func IsXcprettyInstalled() (bool, error) {
	return IsToolInstalled("xcpretty", "")
}

// InstallXcpretty ...
func InstallXcpretty() error {
	cmds, err := rubycommand.GemInstall("xcpretty", "")
	if err != nil {
		return fmt.Errorf("Failed to create command model, error: %s", err)
	}

	for _, cmd := range cmds {
		log.Donef("$ %s", cmd.PrintableCommandArgs())

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Command failed, error: %s", err)
		}
	}

	return nil
}

func parseXcprettyVersionOut(versionOut string) (*version.Version, error) {
	return version.NewVersion(versionOut)
}

// XcprettyVersion ...
func XcprettyVersion() (*version.Version, error) {
	cmd := command.New("xcpretty", "--version")
	versionOut, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseXcprettyVersionOut(versionOut)
}
