package main

import (
	"fmt"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/xcpretty"
	version "github.com/hashicorp/go-version"
)

const (
	installationCheckError = "failed to check if xcpretty is installed"
	installError           = "failed to install xcpretty"
	determineVersionError  = "failed to determine xcpretty version"
)

// InstallXcpretty installs and gets xcpretty version
func InstallXcpretty() (*version.Version, error) {
	fmt.Println()
	log.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := xcpretty.IsInstalled()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", installationCheckError, err)
	} else if !installed {
		log.Warnf(`xcpretty is not installed`)
		fmt.Println()
		log.Printf("Installing xcpretty")

		cmdModelSlice, err := xcpretty.Install()
		if err != nil {
			return nil, fmt.Errorf("%s: %s", installError, err)
		}

		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("%s: %s", installError, err)
			}
		}
	}

	xcprettyVersion, err := xcpretty.Version()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", determineVersionError, err)
	}
	return xcprettyVersion, nil
}

// IsXcprettyInstallationCheckError returns true if the given error has occurred during
// checking whether xcpretty is installed.
func IsXcprettyInstallationCheckError(err error) bool {
	return strings.Contains(err.Error(), installationCheckError)
}
