package step

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-xcode/v2/testresult/xcresult3/model3"
	"github.com/bitrise-steplib/bitrise-step-test-selection/selector"
	"github.com/stretchr/testify/require"
)

func TestWholeTargets(t *testing.T) {
	require.Equal(t, map[string]bool{"UITests": true}, wholeTargets([]string{"UITests", "AppTests/LoginTests/testLogout", " ", ""}))
}

func TestStartTestSelectionShadowWithoutConfiguration(t *testing.T) {
	t.Setenv(selector.ConfigEnvKey, "")
	sh := StartTestSelectionShadow(Config{ProjectPath: filepath.Join(t.TempDir(), "App.xcodeproj"), Scheme: "App"}, log.NewLogger())
	require.Nil(t, sh)
	FinishTestSelectionShadow(nil, "", time.Second, log.NewLogger())
}

func TestStartTestSelectionShadowWithoutBase(t *testing.T) {
	t.Setenv(selector.ConfigEnvKey, `{"mode":"shadow"}`)
	t.Setenv(selector.BaseRefEnvKey, "")
	t.Setenv("BITRISEIO_GIT_BRANCH_DEST", "")
	t.Setenv("ANALYTICS_DISABLED", "true")

	sh := StartTestSelectionShadow(Config{ProjectPath: filepath.Join(t.TempDir(), "App.xcodeproj"), Scheme: "App"}, log.NewLogger())
	require.NotNil(t, sh)
	require.Equal(t, "xcode-test", sh.Report.Step)
	require.Equal(t, selector.ReasonNoBase, sh.Report.ReasonKind)

	FinishTestSelectionShadow(sh, filepath.Join(t.TempDir(), "missing.xcresult"), time.Second, log.NewLogger())
	require.False(t, sh.Report.OutcomesKnown)
}

func TestSummaryOutcomes(t *testing.T) {
	tc := func(result model3.TestResult, secs float64, retries ...model3.TestResult) model3.TestCaseWithRetries {
		c := model3.TestCaseWithRetries{TestCase: model3.TestCase{Result: result, Time: time.Duration(secs * float64(time.Second))}}
		for _, r := range retries {
			c.Retries = append(c.Retries, model3.TestCase{Result: r, Time: time.Second})
		}
		return c
	}
	summary := model3.TestSummary{TestPlans: []model3.TestPlan{{TestBundles: []model3.TestBundle{
		{Name: "CoreTests.xctest", TestSuites: []model3.TestSuite{{TestCases: []model3.TestCaseWithRetries{
			tc(model3.TestResultPassed, 0.5),
			tc(model3.TestResultFailed, 1, model3.TestResultFailed),
			tc(model3.TestResultPassed, 0.5, model3.TestResultFailed),
			tc(model3.TestResultSkipped, 9),
		}}}},
		{Name: "AppTests", TestSuites: []model3.TestSuite{{TestCases: []model3.TestCaseWithRetries{tc(model3.TestResultPassed, 2)}}}},
	}}}}

	require.Equal(t, []selector.UnitOutcome{
		{Key: "CoreTests", Tests: 3, Failed: 1, Flaky: 1, Duration: 4 * time.Second},
		{Key: "AppTests", Tests: 1, Duration: 2 * time.Second},
	}, summaryOutcomes(summary))
}
