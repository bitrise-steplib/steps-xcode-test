package xcodecommand

import (
	"github.com/hashicorp/go-version"
)

type Output struct {
	RawOut   []byte
	ExitCode int
}

type DependencyInstaller interface {
	CheckInstall() (*version.Version, error)
}

type Runner interface {
	Run(workDir string, xcodebuildOpts []string, logFormatterOpts []string) (Output, error)
}
