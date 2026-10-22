// Package selection is the output model of the step: what changed, who owns it, which
// tests run, and why.
package selection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Mode is the top-level decision.
type Mode string

const (
	// ModeSelected means only the listed tests need to run.
	ModeSelected Mode = "selected"
	// ModeFull means every test must run; the tests list still names the candidates.
	ModeFull Mode = "full"
)

// ReasonKind classifies Reason without file names, for aggregate reporting.
type ReasonKind string

const (
	// ReasonGraph means the changed files were mapped onto the project graph.
	ReasonGraph ReasonKind = "graph"
	// ReasonForced means the backend configuration forced a full run.
	ReasonForced ReasonKind = "forced"
	// ReasonNoChanges means no changed file list could be obtained.
	ReasonNoChanges ReasonKind = "no_changes"
	// ReasonIncompleteChanges means the changed file list was incomplete.
	ReasonIncompleteChanges ReasonKind = "incomplete_changes"
	// ReasonGlobalFile means a full-run rule matched a changed file.
	ReasonGlobalFile ReasonKind = "global_file"
	// ReasonUnknownFile means a changed file belongs to no build unit.
	ReasonUnknownFile ReasonKind = "unknown_file"
	// ReasonAnalysisFailed means the project graph could not be built.
	ReasonAnalysisFailed ReasonKind = "analysis_failed"
)

// Version of the output schema.
const Version = 1

// Result is written as JSON and consumed by the test steps.
type Result struct {
	Version      int          `json:"version"`
	Mode         Mode         `json:"mode"`
	Shadow       bool         `json:"shadow"`
	Reason       string       `json:"reason"`
	ReasonKind   ReasonKind   `json:"reason_kind"`
	ChangedFiles ChangedFiles `json:"changed_files"`
	Owners       []Owner      `json:"owners"`
	Affected     []string     `json:"affected_nodes"`
	Tests        []Test       `json:"tests"`
	Stats        Stats        `json:"stats"`
	Errors       []string     `json:"errors,omitempty"`
}

// ChangedFiles describes the input change set.
type ChangedFiles struct {
	Origin   string   `json:"origin"`
	Complete bool     `json:"complete"`
	Base     string   `json:"base,omitempty"`
	Count    int      `json:"count"`
	Files    []string `json:"files"`
}

// Owner records how one changed file was classified.
type Owner struct {
	File    string   `json:"file"`
	Nodes   []string `json:"nodes,omitempty"`
	Ignored bool     `json:"ignored,omitempty"`
	FullRun string   `json:"full_run_rule,omitempty"`
	Unknown bool     `json:"unknown,omitempty"`
}

// Test is one candidate test unit with its decision.
type Test struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Ecosystem string      `json:"ecosystem"`
	Selected  bool        `json:"selected"`
	Reason    string      `json:"reason"`
	Ident     Identifiers `json:"identifiers"`
}

// Identifiers are the framework-specific handles the test steps understand.
type Identifiers struct {
	// OnlyTesting is the xcodebuild -only-testing identifier (the test target name).
	OnlyTesting string `json:"only_testing,omitempty"`
	// GradleModule is the Gradle project path (":feature:home").
	GradleModule string `json:"gradle_module,omitempty"`
	// GradleTask is the test task name to run for the module, when known.
	GradleTask string `json:"gradle_task,omitempty"`
	// PackageDir is the repo-relative directory of a Flutter package.
	PackageDir string `json:"package_dir,omitempty"`
}

// Stats summarises the decision.
type Stats struct {
	Candidates int `json:"candidates"`
	Selected   int `json:"selected"`
	Skipped    int `json:"skipped"`
	MustRun    int `json:"must_run"`
}

// Finalize sorts and counts.
func (r *Result) Finalize() {
	sort.Slice(r.Tests, func(i, j int) bool { return r.Tests[i].ID < r.Tests[j].ID })
	sort.Strings(r.Affected)
	sort.Slice(r.Owners, func(i, j int) bool { return r.Owners[i].File < r.Owners[j].File })
	r.Stats = Stats{Candidates: len(r.Tests)}
	for _, t := range r.Tests {
		if t.Selected {
			r.Stats.Selected++
		} else {
			r.Stats.Skipped++
		}
		if strings.HasPrefix(t.Reason, "must-run") {
			r.Stats.MustRun++
		}
	}
	r.ChangedFiles.Count = len(r.ChangedFiles.Files)
}

// Selected returns the selected tests.
func (r Result) Selected() []Test {
	var out []Test
	for _, t := range r.Tests {
		if t.Selected {
			out = append(out, t)
		}
	}
	return out
}

// Files are the per-ecosystem list files written next to the JSON.
type Files struct {
	JSON             string
	XcodeOnlyTesting string
	GradleModules    string
	GradleTasks      string
	FlutterPackages  string
}

// Write stores the JSON and the per-ecosystem list files in dir. In full mode no list
// file is written, because an empty list means "everything" to the consuming steps.
func Write(dir string, r Result) (Files, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Files{}, err
	}
	files := Files{JSON: filepath.Join(dir, "test-selection.json")}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return Files{}, err
	}
	if err := os.WriteFile(files.JSON, data, 0o644); err != nil {
		return Files{}, err
	}
	if r.Mode == ModeFull {
		return files, nil
	}
	var xcode, gradle, gradleTasks, flutter []string
	for _, t := range r.Selected() {
		if t.Ident.OnlyTesting != "" {
			xcode = append(xcode, t.Ident.OnlyTesting)
		}
		if t.Ident.GradleModule != "" {
			gradle = append(gradle, t.Ident.GradleModule)
		}
		if t.Ident.GradleTask != "" {
			gradleTasks = append(gradleTasks, t.Ident.GradleTask)
		}
		if t.Ident.PackageDir != "" {
			flutter = append(flutter, t.Ident.PackageDir)
		}
	}
	write := func(name string, lines []string) (string, error) {
		if len(lines) == 0 {
			return "", nil
		}
		p := filepath.Join(dir, name)
		return p, os.WriteFile(p, []byte(strings.Join(dedupe(lines), "\n")+"\n"), 0o644)
	}
	if files.XcodeOnlyTesting, err = write("xcode-only-testing.txt", xcode); err != nil {
		return files, err
	}
	if files.GradleModules, err = write("gradle-modules.txt", gradle); err != nil {
		return files, err
	}
	if files.GradleTasks, err = write("gradle-tasks.txt", gradleTasks); err != nil {
		return files, err
	}
	if files.FlutterPackages, err = write("flutter-packages.txt", flutter); err != nil {
		return files, err
	}
	return files, nil
}

// Summary renders the human-readable table for the build log.
func (r Result) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Mode: %s", r.Mode)
	if r.Shadow {
		b.WriteString(" (shadow: consumers run everything and only annotate)")
	}
	fmt.Fprintf(&b, "\nReason: %s\n", r.Reason)
	fmt.Fprintf(&b, "Changed files: %d (%s", r.ChangedFiles.Count, r.ChangedFiles.Origin)
	if !r.ChangedFiles.Complete {
		b.WriteString(", incomplete")
	}
	b.WriteString(")\n")

	if len(r.Owners) > 0 {
		b.WriteString("\nFile → owner\n")
		w := 0
		for _, o := range r.Owners {
			if len(o.File) > w {
				w = len(o.File)
			}
		}
		if w > 70 {
			w = 70
		}
		for _, o := range r.Owners {
			desc := strings.Join(o.Nodes, ", ")
			switch {
			case o.FullRun != "":
				desc = "FULL RUN (rule " + o.FullRun + ")"
			case o.Ignored:
				desc = "ignored"
			case o.Unknown:
				desc = "UNKNOWN owner"
			}
			fmt.Fprintf(&b, "  %-*s  %s\n", w, truncate(o.File, w), desc)
		}
	}

	b.WriteString("\nTests\n")
	for _, t := range r.Tests {
		mark := "skip"
		if t.Selected {
			mark = "RUN "
		}
		fmt.Fprintf(&b, "  %s  %-40s  %s\n", mark, truncate(t.Name, 40), t.Reason)
	}
	fmt.Fprintf(&b, "\n%d of %d test units selected, %d skipped, %d must-run\n", r.Stats.Selected, r.Stats.Candidates, r.Stats.Skipped, r.Stats.MustRun)
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "warning: %s\n", e)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
