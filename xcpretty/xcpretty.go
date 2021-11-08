package xcpretty

import (
	"fmt"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/xcpretty"
	"github.com/hashicorp/go-version"
)

// Installer ...
type Installer interface {
	Install() (*version.Version, error)
}

type installer struct {
	xcpretty xcpretty.Xcpretty
}

// NewInstaller ...
func NewInstaller(xcpretty xcpretty.Xcpretty) Installer {
	return &installer{
		xcpretty: xcpretty,
	}
}

// Install installs and gets xcpretty version
func (i installer) Install() (*version.Version, error) {
	fmt.Println()
	log.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := i.xcpretty.IsInstalled()
	if err != nil {
		return nil, err
	} else if !installed {
		log.Warnf(`xcpretty is not installed`)
		fmt.Println()
		log.Printf("Installing xcpretty")

		cmdModelSlice, err := i.xcpretty.Install()
		if err != nil {
			return nil, fmt.Errorf("failed to install xcpretty: %s", err)
		}

		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to install xcpretty: %s", err)
			}
		}
	}

	xcprettyVersion, err := i.xcpretty.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to determine xcpretty version: %s", err)
	}
	return xcprettyVersion, nil
}
