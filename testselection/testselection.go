// Package testselection reads the Test Selection step's output (test-selection.json)
// and decides which test units this step should run.
package testselection

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Modes written by the Test Selection step.
const (
	ModeSelected = "selected"
	ModeFull     = "full"
	ModeShadow   = "shadow"
)

// Policies for a selection that names no test unit of this step's kind.
const (
	OnEmptyRunAll = "run_all"
	OnEmptySkip   = "skip"
	OnEmptyFail   = "fail"
)

// Selection is the test-selection.json document.
type Selection struct {
	Version int    `json:"version"`
	Mode    string `json:"mode"`
	Shadow  bool   `json:"shadow"`
	Reason  string `json:"reason"`
	Tests   []Test `json:"tests"`
}

// Test is one test unit with the selection decision.
type Test struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Ecosystem   string      `json:"ecosystem"`
	Selected    bool        `json:"selected"`
	Reason      string      `json:"reason"`
	Identifiers Identifiers `json:"identifiers"`
}

// Identifiers are the framework-specific names of a test unit.
type Identifiers struct {
	OnlyTesting  string `json:"only_testing"`
	GradleModule string `json:"gradle_module"`
	GradleTask   string `json:"gradle_task"`
	PackageDir   string `json:"package_dir"`
}

// Action is what the step should do.
type Action int

const (
	// RunAll runs every test, as if there were no selection.
	RunAll Action = iota
	// RunSelected runs only Decision.Tests.
	RunSelected
	// Skip runs nothing and succeeds.
	Skip
)

// Decision is the outcome of Decide.
type Decision struct {
	Action Action
	Tests  []Test
	Reason string
}

// Load reads the selection file. An empty path means no selection.
func Load(path string) (*Selection, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test selection (%s): %w", path, err)
	}
	var sel Selection
	if err := json.Unmarshal(data, &sel); err != nil {
		return nil, fmt.Errorf("parse test selection (%s): %w", path, err)
	}
	if sel.Version != 1 {
		return nil, fmt.Errorf("unsupported test selection version %d in %s", sel.Version, path)
	}
	return &sel, nil
}

// Selected returns the selected tests of the given ecosystems, or of every ecosystem
// when none is given.
func (s *Selection) Selected(ecosystems ...string) []Test {
	var out []Test
	for _, t := range s.Tests {
		if t.Selected && (len(ecosystems) == 0 || contains(ecosystems, t.Ecosystem)) {
			out = append(out, t)
		}
	}
	return out
}

// Decide maps the selection onto what this step should do. onEmpty applies only to a
// selected-mode file that names no test unit of the given ecosystems.
func Decide(sel *Selection, onEmpty string, ecosystems ...string) (Decision, error) {
	switch onEmpty {
	case "", OnEmptyRunAll, OnEmptySkip, OnEmptyFail:
	default:
		return Decision{}, fmt.Errorf("unknown on_empty_selection value %q, expected run_all, skip or fail", onEmpty)
	}
	if sel == nil {
		return Decision{Action: RunAll, Reason: "no test selection input"}, nil
	}
	switch sel.Mode {
	case ModeFull:
		return Decision{Action: RunAll, Reason: "full run: " + sel.Reason}, nil
	case ModeShadow:
		return Decision{Action: RunAll, Reason: "shadow mode: every test runs"}, nil
	case ModeSelected:
	default:
		return Decision{}, fmt.Errorf("unknown test selection mode %q", sel.Mode)
	}
	tests := sel.Selected(ecosystems...)
	if len(tests) > 0 {
		return Decision{Action: RunSelected, Tests: tests, Reason: sel.Reason}, nil
	}
	switch onEmpty {
	case OnEmptySkip:
		return Decision{Action: Skip, Reason: "no test unit is affected by this change"}, nil
	case OnEmptyFail:
		return Decision{}, fmt.Errorf("no test unit is affected by this change and on_empty_selection is %q", OnEmptyFail)
	default:
		return Decision{Action: RunAll, Reason: "no test unit is affected by this change, on_empty_selection is run_all"}, nil
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
