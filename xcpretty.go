package main

import (
	"fmt"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-xcode/xcpretty"
	version "github.com/hashicorp/go-version"
)

// InstallXcpretty installs and gets xcpretty version
func InstallXcpretty() (*version.Version, error) {
	fmt.Println()
	log.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := xcpretty.IsInstalled()
	if err != nil {
		return nil, fmt.Errorf("failed to check if xcpretty is installed, error: %s", err)
	} else if !installed {
		log.Warnf(`xcpretty is not installed`)
		fmt.Println()
		log.Printf("Installing xcpretty")

		cmdModelSlice, err := xcpretty.Install()
		if err != nil {
			return nil, fmt.Errorf("failed to install xcpretty, error: %s", err)
		}

		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to install xcpretty, error: %s", err)
			}
		}
	}

	xcprettyVersion, err := xcpretty.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to determine xcpretty version, error: %s", err)
	}
	return xcprettyVersion, nil
}
