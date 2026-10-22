package step

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/testresult/xcresult3"
	"github.com/bitrise-io/go-xcode/v2/testresult/xcresult3/model3"
	"github.com/bitrise-steplib/bitrise-step-test-selection/selector"
)

// StartTestSelectionShadow works out which test targets a change-based test selection
// would have run. It never changes what the step runs, and it only acts when the
// backend injects a shadow configuration; otherwise it returns nil.
func StartTestSelectionShadow(cfg Config, logger log.Logger) *selector.Shadow {
	skipped := wholeTargets(cfg.SkipTesting)
	sh, ok := selector.StartShadow(selector.ShadowOptions{
		Step: "xcode-test",
		Config: selector.Config{
			ProjectType: "xcode",
			ProjectPath: cfg.ProjectPath,
			Scheme:      cfg.Scheme,
		},
		WorkDir: filepath.Dir(cfg.ProjectPath),
		Keep:    func(t selector.Test, _ string) bool { return !skipped[t.Name] },
		Logf: func(format string, v ...interface{}) {
			logger.Println()
			logger.Infof(format, v...)
		},
		Debugf: logger.Debugf,
	})
	if !ok {
		return nil
	}
	return sh
}

// FinishTestSelectionShadow reports each test target's results so the backend can tell
// what a selection would have saved and whether it would have skipped a failure.
func FinishTestSelectionShadow(sh *selector.Shadow, xcresultPath string, testRun time.Duration, logger log.Logger) {
	if sh == nil {
		return
	}
	outcomes, err := xcresultOutcomes(xcresultPath)
	if err != nil {
		logger.Debugf("Test selection shadow run: cannot read the test results: %s", err)
	}
	sh.Finish(outcomes, testRun)
}

// xcresultOutcomes sums the xcresult's test cases per test target. A target's duration
// is its test cases' time including retries, so building is not counted and the saving
// is a lower bound.
func xcresultOutcomes(xcresultPath string) ([]selector.UnitOutcome, error) {
	if _, err := os.Stat(xcresultPath); err != nil {
		return nil, err
	}
	data, err := xcresult3.ParseTestResults(xcresultPath)
	if err != nil {
		return nil, err
	}
	summary, _, err := model3.Convert(data)
	if err != nil {
		return nil, err
	}
	return summaryOutcomes(*summary), nil
}

func summaryOutcomes(summary model3.TestSummary) []selector.UnitOutcome {
	var keys []string
	byTarget := map[string]*selector.UnitOutcome{}
	for _, plan := range summary.TestPlans {
		for _, bundle := range plan.TestBundles {
			key := strings.TrimSuffix(bundle.Name, ".xctest")
			o, ok := byTarget[key]
			if !ok {
				o = &selector.UnitOutcome{Key: key}
				byTarget[key] = o
				keys = append(keys, key)
			}
			for _, suite := range bundle.TestSuites {
				for _, tc := range suite.TestCases {
					addTestCase(o, tc)
				}
			}
		}
	}
	outcomes := make([]selector.UnitOutcome, 0, len(keys))
	for _, k := range keys {
		outcomes = append(outcomes, *byTarget[k])
	}
	return outcomes
}

func addTestCase(o *selector.UnitOutcome, tc model3.TestCaseWithRetries) {
	if tc.Result == model3.TestResultSkipped {
		return
	}
	o.Tests++
	o.Duration += tc.Time
	failedBefore := false
	for _, r := range tc.Retries {
		o.Duration += r.Time
		failedBefore = failedBefore || r.Result == model3.TestResultFailed
	}
	switch {
	case tc.Result == model3.TestResultFailed:
		o.Failed++
	case failedBefore:
		o.Flaky++
	}
}

// wholeTargets returns the targets a skip list excludes entirely; an identifier with
// a class or method only skips part of its target.
func wholeTargets(identifiers []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range identifiers {
		id = strings.TrimSpace(id)
		if id != "" && !strings.Contains(id, "/") {
			out[id] = true
		}
	}
	return out
}
