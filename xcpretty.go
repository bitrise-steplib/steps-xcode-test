package main

import (
	"fmt"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/xcpretty"
	version "github.com/hashicorp/go-version"
)

type xcprettyInstallationCheckError struct {
	Message string
}

func (e *xcprettyInstallationCheckError) Error() string {
	return fmt.Sprintf("failed to check if xcpretty is installed: %s", e.Message)
}

func newXcprettyInstallationCheckError(message string) *xcprettyInstallationCheckError {
	return &xcprettyInstallationCheckError{Message: message}
}

func isXcprettyInstallationCheckError(err error) bool {
	_, ok := err.(*xcprettyInstallationCheckError)
	return ok
}

// InstallXcpretty installs and gets xcpretty version
func InstallXcpretty() (*version.Version, error) {
	fmt.Println()
	log.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := xcpretty.IsInstalled()
	if err != nil {
		return nil, newXcprettyInstallationCheckError(err.Error())
	} else if !installed {
		log.Warnf(`xcpretty is not installed`)
		fmt.Println()
		log.Printf("Installing xcpretty")

		cmdModelSlice, err := xcpretty.Install()
		if err != nil {
			return nil, fmt.Errorf("failed to install xcpretty: %s", err)
		}

		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to install xcpretty: %s", err)
			}
		}
	}

	xcprettyVersion, err := xcpretty.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to determine xcpretty version: %s", err)
	}
	return xcprettyVersion, nil
}

func handleXcprettyInstallError(err error) (string, error) {
	if isXcprettyInstallationCheckError(err) {
		return "", err
	}

	log.Warnf("%s", err)
	log.Printf("Switching to xcodebuild for output tool")
	return xcodebuildTool, nil
}
