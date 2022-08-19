package xcoderunner

import (
	"github.com/bitrise-io/go-steputils/v2/ruby"
	command "github.com/bitrise-io/go-utils/v2/command"
	version "github.com/hashicorp/go-version"
)

type xcprettyManager interface {
	isDepInstalled() (bool, error)
	installDep() []command.Command
	depVersion() (*version.Version, error)
}

type xcpretty struct {
	commandFactory     command.Factory
	rubyEnv            ruby.Environment
	rubyCommandFactory ruby.CommandFactory
}

func (c *xcpretty) isDepInstalled() (bool, error) {
	return c.rubyEnv.IsGemInstalled("xcpretty", "")
}

func (c *xcpretty) installDep() []command.Command {
	cmds := c.rubyCommandFactory.CreateGemInstall("xcpretty", "", false, false, nil)
	return cmds
}

func (c *xcpretty) depVersion() (*version.Version, error) {
	cmd := c.commandFactory.Create("xcpretty", []string{"--version"}, nil)

	versionOut, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}

	return version.NewVersion(versionOut)
}
