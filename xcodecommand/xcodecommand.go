package xcodecommand

import (
	"github.com/hashicorp/go-version"
)

type Output struct {
	RawOut           []byte
	DidWriteToStdOut bool
	ExitCode         int
}

type DependencyInstaller interface {
	Install() (*version.Version, error)
}

type Runner interface {
	Run(workDir string, xcodebuildArgs []string, toolArgs []string) (Output, error)
}
