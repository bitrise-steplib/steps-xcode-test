package selector

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/v2/analytics"
	"github.com/bitrise-io/go-utils/v2/log"

	"github.com/bitrise-steplib/bitrise-step-test-selection/internal/selection"
)

const (
	// ConfigEnvKey carries the backend's selection configuration into the build.
	ConfigEnvKey = "BITRISE_TEST_SELECTION_CONFIG_JSON"
	// BaseRefEnvKey is the commit or branch the backend wants the change diffed against.
	BaseRefEnvKey = "BITRISE_TEST_SELECTION_BASE_REF"
	// ReportVersion is the schema version of Report.
	ReportVersion = 1

	defaultShadowTimeout = 60 * time.Second
)

// Decisions a shadow report can carry besides the selection modes.
const (
	DecisionError = "error"
)

// Reason kinds that only the shadow runner produces.
const (
	ReasonNoBase    ReasonKind = "no_base"
	ReasonTimeout   ReasonKind = "timeout"
	ReasonPanic     ReasonKind = "panic"
	ReasonNoProject ReasonKind = "no_project"
	ReasonFailed    ReasonKind = "failed"
)

// BackendConfig is the part of BITRISE_TEST_SELECTION_CONFIG_JSON the test steps read.
// The whole JSON is also handed to Run, so its rule lists and force_full_run apply.
type BackendConfig struct {
	Mode          string   `json:"mode"`
	BaseRef       string   `json:"base_ref"`
	BaseBuildSlug string   `json:"base_build_slug"`
	SampleRate    *float64 `json:"sample_rate"`
	// SampleRates overrides SampleRate per consuming step, keyed by ShadowOptions.Step,
	// because the Gradle analysis costs far more than the Xcode or Flutter one.
	SampleRates    map[string]float64 `json:"sample_rates"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	// ForceFullRun is set for builds that always run every test (the default branch,
	// release branches, scheduled and manual builds).
	ForceFullRun       bool   `json:"force_full_run"`
	ForceFullRunReason string `json:"force_full_run_reason"`
}

// SampleRateFor returns the sample rate that applies to step.
func (c BackendConfig) SampleRateFor(step string) *float64 {
	if rate, ok := c.SampleRates[step]; ok {
		return &rate
	}
	return c.SampleRate
}

// ShadowConfig returns the backend configuration when it asks for a shadow run.
func ShadowConfig(getenv func(string) string) (BackendConfig, bool) {
	raw := strings.TrimSpace(getenv(ConfigEnvKey))
	if raw == "" {
		return BackendConfig{}, false
	}
	var bc BackendConfig
	if err := json.Unmarshal([]byte(raw), &bc); err != nil {
		return BackendConfig{}, false
	}
	return bc, bc.Mode == "shadow"
}

// Sampled reports whether this build falls into the sample. The build slug makes the
// decision stable across the steps of one build.
func Sampled(rate *float64, buildSlug string) bool {
	if rate == nil || *rate >= 1 {
		return true
	}
	if *rate <= 0 {
		return false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(buildSlug))
	return float64(h.Sum32())/float64(1<<32) < *rate
}

// Report is what a test step publishes about a shadow selection. It names test units
// but never files, because it leaves the customer's build.
type Report struct {
	Version       int        `json:"version"`
	Step          string     `json:"step"`
	Decision      string     `json:"decision"`
	ReasonKind    ReasonKind `json:"reason_kind"`
	Base          string     `json:"base,omitempty"`
	BaseBuildSlug string     `json:"base_build_slug,omitempty"`
	ChangedFiles  int        `json:"changed_files"`
	// DurationMS is what the analysis cost, so it can be set against the saving.
	DurationMS int64    `json:"duration_ms"`
	Candidates int      `json:"candidates"`
	Selected   []string `json:"selected"`
	Skipped    []string `json:"skipped"`

	// Filled in by Finish from what the step's test run did.
	Units             []UnitReport `json:"units"`
	OutcomesKnown     bool         `json:"outcomes_known"`
	TestRunMS         int64        `json:"test_run_ms"`
	EstimatedSavingMS int64        `json:"estimated_saving_ms"`
	// EstimatedSavingByUnitsMS splits the test run's wall time evenly across the units
	// that ran. EstimatedSavingMS only counts test case time, which leaves out each
	// unit's compilation and test host launch; the real saving lies between the two.
	EstimatedSavingByUnitsMS int64    `json:"estimated_saving_by_units_ms"`
	MissedFailure            bool     `json:"missed_failure"`
	MissedUnits              []string `json:"missed_units"`
	FlakyInSkipped           int      `json:"flaky_in_skipped"`
}

// UnitReport is one test unit with the decision and what its tests did.
type UnitReport struct {
	Name string `json:"name"`
	key  string
	// Selected is false for a unit the analysis did not know (Mapped false), because a
	// selection would not have run it either.
	Selected   bool  `json:"selected"`
	Mapped     bool  `json:"mapped"`
	Ran        bool  `json:"ran"`
	Tests      int   `json:"tests"`
	Failed     int   `json:"failed"`
	Flaky      int   `json:"flaky"`
	DurationMS int64 `json:"duration_ms"`
}

// UnitOutcome is what the step observed for one test unit in its test run.
type UnitOutcome struct {
	// Key identifies the unit the way UnitKey does.
	Key   string
	Tests int
	// Failed counts tests whose final attempt failed.
	Failed int
	// Flaky counts tests that failed and then passed on a retry.
	Flaky    int
	Duration time.Duration
}

// UnitKey is the step-side name of a test unit: the Xcode test target, the Gradle
// project path (":feature:home") or the Flutter package directory.
func UnitKey(t Test) string {
	switch {
	case t.Ident.OnlyTesting != "":
		return t.Ident.OnlyTesting
	case t.Ident.GradleModule != "":
		return t.Ident.GradleModule
	case t.Ident.PackageDir != "":
		return t.Ident.PackageDir
	}
	return t.Name
}

// Shadow is a shadow selection started before the step's tests and finished with
// what they did.
type Shadow struct {
	Report  Report
	repoDir string
	getenv  func(string) string
	logf    func(format string, v ...interface{})
	debugf  func(format string, v ...interface{})
}

// RepoDir is the absolute repository root the units' paths are relative to.
func (s *Shadow) RepoDir() string { return s.repoDir }

// ShadowOptions describe the consuming step's project.
type ShadowOptions struct {
	// Step names the consuming step in the report, for example "xcode-test".
	Step string
	// Config holds the project inputs (ProjectType, ProjectPath, Scheme,
	// GradleProjectDir, FlutterProjectDir...). The runner fills in the base ref, the
	// backend configuration and the safe defaults.
	Config Config
	// WorkDir is used to find the repository root when Config.RepoDir is empty.
	WorkDir string
	// Keep limits the report to the test units the step runs; repoDir is the absolute
	// repository root the units' paths are relative to. Nil keeps every unit.
	Keep func(t Test, repoDir string) bool
	// Getenv defaults to os.Getenv.
	Getenv func(string) string
	// Logf prints the one-line summary; Debugf gets the analysis details.
	Logf, Debugf func(format string, v ...interface{})
}

// run is replaced in tests.
var run = Run

// StartShadow computes the selection a test step would have made, without changing
// what it runs. ok is false when the backend did not ask for a shadow run or the build
// is not sampled. It never panics and never takes longer than the configured timeout.
func StartShadow(opts ShadowOptions) (sh *Shadow, ok bool) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...interface{}) {}
	}
	debugf := opts.Debugf
	if debugf == nil {
		debugf = func(string, ...interface{}) {}
	}

	bc, enabled := ShadowConfig(getenv)
	if !enabled {
		return nil, false
	}
	if !Sampled(bc.SampleRateFor(opts.Step), getenv("BITRISE_BUILD_SLUG")) {
		debugf("Test selection shadow run: this build is not in the sample")
		return nil, false
	}

	start := time.Now()
	sh = &Shadow{
		Report: Report{Version: ReportVersion, Step: opts.Step, BaseBuildSlug: bc.BaseBuildSlug, Selected: []string{}, Skipped: []string{}},
		getenv: getenv,
		logf:   logf,
		debugf: debugf,
	}
	finish := func(decision string, kind ReasonKind, detail string) (*Shadow, bool) {
		sh.Report.Decision = decision
		sh.Report.ReasonKind = kind
		sh.Report.DurationMS = time.Since(start).Milliseconds()
		logf("Test selection (shadow, nothing was skipped): %s", detail)
		return sh, true
	}

	// The decision is "everything" whatever changed, so the diff and the project graph
	// would only cost time; the report still goes out so these builds are counted.
	if bc.ForceFullRun {
		reason := firstNonEmpty(bc.ForceFullRunReason, "full run forced by the build configuration")
		return finish(string(ModeFull), selection.ReasonForced, reason+"; nothing was analysed")
	}

	baseRef := firstNonEmpty(bc.BaseRef, getenv(BaseRefEnvKey), getenv("BITRISEIO_GIT_BRANCH_DEST"))
	if baseRef == "" {
		return finish(string(ModeFull), ReasonNoBase, "no base commit or target branch to compare with")
	}

	cfg := opts.Config
	if cfg.ProjectType == "xcode" && strings.TrimSpace(cfg.ProjectPath) == "" {
		return finish(DecisionError, ReasonNoProject, "no Xcode project or workspace path is known to this step")
	}
	cfg.BaseRef = baseRef
	cfg.BackendConfig = getenv(ConfigEnvKey)
	cfg.Mode = "shadow"
	// The env list carries no completeness flag yet, so the diff always comes from git.
	cfg.ChangedFiles = ""
	cfg.ChangedFilesComplete = ""
	cfg.OnError = "full"
	if cfg.OnUnknownFile == "" {
		cfg.OnUnknownFile = "full"
	}
	if cfg.ProjectType == "" {
		cfg.ProjectType = "auto"
	}

	timeout := defaultShadowTimeout
	if bc.TimeoutSeconds > 0 {
		timeout = time.Duration(bc.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if cfg.RepoDir == "" {
		root, err := (execGit{ctx}).Run(firstNonEmpty(opts.WorkDir, getenv("BITRISE_SOURCE_DIR"), "."), "rev-parse", "--show-toplevel")
		if err != nil || root == "" {
			debugf("cannot find the repository root: %v", err)
			return finish(DecisionError, ReasonNoProject, "the working directory is not a git repository")
		}
		cfg.RepoDir = root
	}
	cfg.RepoDir = canonical(cfg.RepoDir)
	sh.repoDir = cfg.RepoDir
	// The analysers take project directories relative to the repository root, while a
	// test step usually knows its project as an absolute path.
	for _, dir := range []*string{&cfg.GradleProjectDir, &cfg.FlutterProjectDir} {
		if filepath.IsAbs(*dir) {
			rel, inRepo := RelPath(cfg.RepoDir, *dir)
			if !inRepo {
				return finish(DecisionError, ReasonNoProject, "the project directory is outside the repository")
			}
			*dir = rel
		}
	}

	type outcome struct {
		res      Result
		err      error
		panicked interface{}
	}
	done := make(chan outcome, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- outcome{panicked: p}
			}
		}()
		res, err := run(cfg, Deps{Git: execGit{ctx}, Cmd: execCmd{ctx}, Logf: debugf, Debugf: debugf})
		done <- outcome{res: res, err: err}
	}()

	var o outcome
	select {
	case o = <-done:
	case <-ctx.Done():
		return finish(DecisionError, ReasonTimeout, fmt.Sprintf("the analysis did not finish in %s", timeout))
	}
	if o.panicked != nil {
		debugf("test selection panicked: %v", o.panicked)
		return finish(DecisionError, ReasonPanic, "the analysis failed unexpectedly")
	}
	if o.err != nil {
		debugf("test selection failed: %v", o.err)
		return finish(DecisionError, ReasonFailed, "the analysis failed: "+o.err.Error())
	}

	res := o.res
	rep := &sh.Report
	rep.Base = res.ChangedFiles.Base
	rep.ChangedFiles = res.ChangedFiles.Count
	for _, t := range res.Tests {
		if opts.Keep != nil && !opts.Keep(t, cfg.RepoDir) {
			continue
		}
		rep.Candidates++
		rep.Units = append(rep.Units, UnitReport{Name: t.Name, key: UnitKey(t), Selected: t.Selected, Mapped: true})
		if t.Selected {
			rep.Selected = append(rep.Selected, t.Name)
		} else {
			rep.Skipped = append(rep.Skipped, t.Name)
		}
	}
	detail := fmt.Sprintf("would have run %d of %d test units (%s)", len(rep.Selected), rep.Candidates, res.Reason)
	if len(rep.Skipped) > 0 {
		detail += "; would have skipped: " + strings.Join(rep.Skipped, ", ")
	}
	return finish(string(res.Mode), res.ReasonKind, detail)
}

// Finish adds what the step's tests did to the report and publishes it. outcomes is
// nil when the step could not read its results; testRun is the wall time of the
// step's test run.
func (s *Shadow) Finish(outcomes []UnitOutcome, testRun time.Duration) {
	if s == nil {
		return
	}
	s.complete(outcomes, testRun)
	s.publish()
}

func (s *Shadow) complete(outcomes []UnitOutcome, testRun time.Duration) {
	rep := &s.Report
	rep.TestRunMS = testRun.Milliseconds()
	rep.OutcomesKnown = outcomes != nil
	rep.MissedUnits = []string{}

	byKey := map[string]int{}
	for i, u := range rep.Units {
		byKey[u.key] = i
	}
	for _, o := range outcomes {
		i, known := byKey[o.Key]
		if !known {
			// Outside a selection (forced, no base, error) every unit runs.
			rep.Units = append(rep.Units, UnitReport{Name: o.Key, key: o.Key, Selected: rep.Decision != string(ModeSelected)})
			i = len(rep.Units) - 1
			byKey[o.Key] = i
		}
		u := &rep.Units[i]
		u.Ran = true
		u.Tests += o.Tests
		u.Failed += o.Failed
		u.Flaky += o.Flaky
		u.DurationMS += o.Duration.Milliseconds()
	}

	// Only a selection that would have skipped something can save time or miss a failure.
	if rep.Decision != string(ModeSelected) || !rep.OutcomesKnown {
		return
	}
	anySelectedRan := false
	ran, skippedRan := 0, 0
	for _, u := range rep.Units {
		if !u.Ran {
			continue
		}
		ran++
		if u.Selected {
			anySelectedRan = true
			continue
		}
		skippedRan++
		rep.EstimatedSavingMS += u.DurationMS
		rep.FlakyInSkipped += u.Flaky
		if u.Failed > 0 {
			rep.MissedFailure = true
			rep.MissedUnits = append(rep.MissedUnits, u.Name)
		}
	}
	// With nothing selected the step itself would have been skipped, simulator boot and
	// Gradle configuration included, not just the test cases.
	if !anySelectedRan {
		rep.EstimatedSavingMS = rep.TestRunMS
		rep.EstimatedSavingByUnitsMS = rep.TestRunMS
		return
	}
	rep.EstimatedSavingByUnitsMS = rep.TestRunMS * int64(skippedRan) / int64(ran)
}

// Event names on the step analytics channel.
const (
	ShadowEvent        = "test_selection_shadow"
	MissedFailureEvent = "test_selection_missed_failure"
)

// newTracker is replaced in tests.
var newTracker = defaultNewTracker

func defaultNewTracker(getenv func(string) string) analytics.Tracker {
	if getenv("ANALYTICS_DISABLED") == "true" {
		return nil
	}
	logger := log.NewLogger()
	logger.EnableDebugLog(getenv(VerboseEnvKey) == "true")
	// The default client waits 30 s for a slow endpoint; a shadow run must not hold up
	// the customer's build that long.
	return analytics.NewTracker(analytics.NewDefaultClient(logger, publishTimeout), publishTimeout)
}

// VerboseEnvKey prints the published report and the analytics client's delivery errors,
// which are otherwise only debug logs, so that a test app can verify the pipeline.
const VerboseEnvKey = "BITRISE_TEST_SELECTION_SHADOW_VERBOSE"

const publishTimeout = 5 * time.Second

func (s *Shadow) publish() {
	rep := s.Report
	common := analytics.Properties{
		"step_id":         rep.Step,
		"build_slug":      s.getenv("BITRISE_BUILD_SLUG"),
		"app_slug":        s.getenv("BITRISE_APP_SLUG"),
		"workspace_id":    s.getenv("BITRISE_WORKSPACE_ID"),
		"version":         rep.Version,
		"decision":        rep.Decision,
		"reason_kind":     string(rep.ReasonKind),
		"base":            rep.Base,
		"base_build_slug": rep.BaseBuildSlug,
	}
	units, _ := json.Marshal(rep.Units)
	shadow := common.Merge(analytics.Properties{
		"changed_files":                rep.ChangedFiles,
		"duration_ms":                  rep.DurationMS,
		"candidates":                   rep.Candidates,
		"selected_count":               len(rep.Selected),
		"skipped_count":                len(rep.Skipped),
		"selected":                     strings.Join(rep.Selected, ","),
		"skipped":                      strings.Join(rep.Skipped, ","),
		"units":                        string(units),
		"outcomes_known":               rep.OutcomesKnown,
		"test_run_ms":                  rep.TestRunMS,
		"estimated_saving_ms":          rep.EstimatedSavingMS,
		"estimated_saving_by_units_ms": rep.EstimatedSavingByUnitsMS,
		"missed_failure":               rep.MissedFailure,
		"missed_units":                 strings.Join(rep.MissedUnits, ","),
		"flaky_in_skipped":             rep.FlakyInSkipped,
	})
	if data, err := json.Marshal(rep); err == nil {
		if s.getenv(VerboseEnvKey) == "true" {
			s.logf("Test selection shadow report: %s", data)
		} else {
			s.debugf("Test selection shadow report: %s", data)
		}
	}

	tracker := newTracker(s.getenv)
	if tracker == nil {
		return
	}
	tracker.Enqueue(ShadowEvent, shadow)
	for _, u := range rep.Units {
		if u.Ran && !u.Selected && u.Failed > 0 && rep.MissedFailure {
			tracker.Enqueue(MissedFailureEvent, common.Merge(analytics.Properties{
				"unit":   u.Name,
				"mapped": u.Mapped,
				"tests":  u.Tests,
				"failed": u.Failed,
				"flaky":  u.Flaky,
			}))
		}
	}
	tracker.Wait()
}

// execGit and execCmd run commands bound to the shadow timeout, so a slow git fetch
// or Gradle configuration is killed rather than left running next to the tests.
type execGit struct{ ctx context.Context }

func (g execGit) Run(dir string, args ...string) (string, error) {
	return runCommand(g.ctx, dir, "git", args...)
}

type execCmd struct{ ctx context.Context }

func (c execCmd) Run(dir, name string, args ...string) (string, error) {
	return runCommand(c.ctx, dir, name, args...)
}

func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RelPath returns path relative to repoDir in slash form, resolving symlinks on both
// sides (git reports /private/var where the step sees /var on macOS). ok is false when
// path is outside repoDir.
func RelPath(repoDir, path string) (string, bool) {
	rel, err := filepath.Rel(canonical(repoDir), canonical(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func canonical(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
