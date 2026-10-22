// Package selector is the importable form of the Test Selection step. The test steps
// call it to compute a selection themselves, without the step in the workflow.
package selector

import (
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/changes"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/gradle"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/selection"
	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/step"
)

type (
	// Config is the analysis input; the fields mirror the Test Selection step's inputs.
	Config = step.Config
	// Deps are the analysis side effects.
	Deps = step.Deps
	// GitRunner runs git in a directory.
	GitRunner = changes.Runner
	// CmdRunner runs a command in a directory (the Gradle wrapper).
	CmdRunner = gradle.Runner
	// Result is the selection, as the Test Selection step writes it.
	Result = selection.Result
	// Test is one candidate test unit with its decision.
	Test = selection.Test
	// Identifiers are the framework-specific names of a test unit.
	Identifiers = selection.Identifiers
	// Mode is the top-level decision.
	Mode = selection.Mode
	// ReasonKind classifies the decision without file names.
	ReasonKind = selection.ReasonKind
	// Files are the paths Write produced.
	Files = selection.Files
)

const (
	ModeSelected = selection.ModeSelected
	ModeFull     = selection.ModeFull
)

// Run computes the selection. It only returns an error when cfg.OnError is "fail";
// every analysis problem is otherwise reported as a full run inside the result.
func Run(cfg Config, deps Deps) (Result, error) {
	return step.Run(cfg, deps)
}

// Write stores the selection JSON and the per-ecosystem list files in dir.
func Write(dir string, r Result) (Files, error) {
	return selection.Write(dir, r)
}
