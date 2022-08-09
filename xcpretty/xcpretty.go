package xcpretty

import (
	"fmt"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/xcpretty"
	"github.com/hashicorp/go-version"
)

// Installer ...
type Installer interface {
	Install() (*version.Version, error)
}

type installer struct {
	xcpretty xcpretty.Xcpretty
	logger   log.Logger
}

// NewInstaller ...
func NewInstaller(xcpretty xcpretty.Xcpretty, logger log.Logger) Installer {
	return &installer{
		xcpretty: xcpretty,
		logger:   logger,
	}
}

// Install installs and gets xcpretty version
func (i installer) Install() (*version.Version, error) {
	fmt.Println()

	i.logger.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := i.xcpretty.IsInstalled()
	if err != nil {
		return nil, err
	} else if !installed {
		i.logger.Warnf(`xcpretty is not installed`)
		fmt.Println()
		i.logger.Printf("Installing xcpretty")

		cmdModelSlice, err := i.xcpretty.Install()
		if err != nil {
			return nil, fmt.Errorf("failed to create xcpretty install commands: %w", err)
		}

		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to run xcpretty install command (%s): %w", cmd.PrintableCommandArgs(), err)
			}
		}
	}

	xcprettyVersion, err := i.xcpretty.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to get xcpretty version: %w", err)
	}
	return xcprettyVersion, nil
}
