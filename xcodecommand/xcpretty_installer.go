package xcodecommand

import (
	"fmt"

	"github.com/bitrise-io/go-steputils/v2/ruby"
	command "github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
	version "github.com/hashicorp/go-version"
)

type xcprettyDependencyManager struct {
	logger   log.Logger
	xcpretty xcprettyManager
}

func NewXcprettyDependencyManager(logger log.Logger, commandFactory command.Factory, rubyCommandFactory ruby.CommandFactory, rubyEnv ruby.Environment) DependencyInstaller {
	return &xcprettyDependencyManager{
		logger: logger,
		xcpretty: &xcpretty{
			commandFactory:     commandFactory,
			rubyEnv:            rubyEnv,
			rubyCommandFactory: rubyCommandFactory,
		},
	}
}

func (c *xcprettyDependencyManager) Install() (*version.Version, error) {
	c.logger.Println()
	c.logger.Infof("Checking if output tool (xcpretty) is installed")

	installed, err := c.xcpretty.isDepInstalled()
	if err != nil {
		return nil, err
	} else if !installed {
		c.logger.Warnf(`xcpretty is not installed`)
		fmt.Println()
		c.logger.Printf("Installing xcpretty")

		cmdModelSlice := c.xcpretty.installDep()
		for _, cmd := range cmdModelSlice {
			if err := cmd.Run(); err != nil {
				return nil, fmt.Errorf("failed to run xcpretty install command (%s): %w", cmd.PrintableCommandArgs(), err)
			}
		}
	}

	xcprettyVersion, err := c.xcpretty.depVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get xcpretty version: %w", err)
	}

	return xcprettyVersion, nil
}
