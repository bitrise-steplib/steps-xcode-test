package testresults

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bitrise-tools/go-xcode/plistutil"
)

// ScreenshotsType descdribes the screenshot atttachment's type
type ScreenshotsType string

// const ...
const (
	ScreenshotsLegacy        ScreenshotsType = "ScreenshotsLegacy"
	ScreenshotsAsAttachments ScreenshotsType = "ScreenshotsAsAttachments"
	ScreenshotsNone          ScreenshotsType = "ScreenshotsNone"
)

// FailureSummaries describes a failed test
type FailureSummaries struct {
	FileName             string `plist:"FileName"`
	LineNumber           string `plist:"LineNumber"`
	Message              string `plist:"Message"`
	IsPerformanceFailure bool   `plist:"PerformanceFailure"`
}

// ActivityScreenshot describes a screenshot attached to a Activity
type ActivityScreenshot struct {
	FilePath  string
	Timestamp time.Time
}

// Activity describes a single xcode UI test activity
type Activity struct {
	Title         string
	UUID          string
	Screenhsot    *ActivityScreenshot
	SubActivities []Activity
}

// TestResult describes a single UI test's output
type TestResult struct {
	ID          string
	TestStatus  string
	FailureInfo *[]FailureSummaries
	Activities  []Activity
}

// type TestSummary {
// 	TestCase Test `plist:"Tests"`
// }

// type TestSummaries {
// 	Summaries TestSummary `plist:"TestableSummaries"`
// }

/*
// TestSummaries ...
type TestSummaries struct {
	Type                     ScreenshotsType
	Content                  string
	TestItemsWithScreenshots []map[string]interface{}
}
*/

// New parses an *_TestSummaries.plist and returns an array containing test results and screenshots
func New(testSummariesPth string) (*[]TestResult, error) {
	testSummariesPlistData, err := plistutil.NewPlistDataFromFile(testSummariesPth)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TestSummaries file: %s, error: %s", testSummariesPth, err)
	}

	return parseTestSummaries(testSummariesPlistData)
}

func parseFailureSummaries(failureSummariesData []plistutil.PlistData) (*[]FailureSummaries, error) {
	var failureSummaries = make([]FailureSummaries, len(failureSummariesData))
	for i, failureSummary := range failureSummariesData {

		fileName, found := failureSummary.GetString("FileName")
		if !found {
			return nil, fmt.Errorf("key FileName not found for FailureSummaries: %s", failureSummariesData)
		}
		lineNumber, found := failureSummary.GetString("LineNumber")
		if !found {
			return nil, fmt.Errorf("key lineNumber not found for FailureSummaries: %s", failureSummariesData)
		}
		message, found := failureSummary.GetString("Message")
		if !found {
			return nil, fmt.Errorf("key Message not found for FailureSummaries: %s", failureSummariesData)
		}
		isPerformanceFailure, found := failureSummary.GetBool("PerformanceFailure")
		if !found {
			return nil, fmt.Errorf("key PerformanceFailure not found for FailureSummaries: %s", failureSummariesData)
		}
		failureSummaries[i] = FailureSummaries{
			FileName:             fileName,
			LineNumber:           lineNumber,
			Message:              message,
			IsPerformanceFailure: isPerformanceFailure,
		}
	}
	return &failureSummaries, nil
}

func parseSceenshot(activitySummary plistutil.PlistData) (*ActivityScreenshot, error) {
		// getAttachmentType := func(item map[string]interface{}) ScreenshotsType {
	// 	value, found := item["Attachments"]
	// 	if found {
	// 		return ScreenshotsAsAttachments
	// 	}
	// 	value, found = item["HasScreenshotData"]
	// 	if found {
	// 		hasScreenshot, casted := value.(bool)
	// 		if casted && hasScreenshot {
	// 			return ScreenshotsLegacy
	// 		}
	// 	}
	// 	return ScreenshotsNone
	// }
}

func parseActivites(activitySummariesData []plistutil.PlistData) ([]Activity, error) {
	var activities = make([]Activity, len(activitySummariesData))
	for i, activity := range activitySummariesData {
		title, found := activity.GetString("Title")
		if !found {
			return nil, fmt.Errorf("key Title not found for activity: %s", activity)
		}
		UUID, found := activity.GetString("UUID")
		if !found {
			return nil, fmt.Errorf("key UUID not found for activity")
		}
		var subActivities []Activity
		if subActivitiesData, err := activity.GetMapStringInterfaceArray("SubActivities") && err != nil {
			if subActivities, err := parseActivites(subActivitiesData) && err != nil {
				return nil, err
			}
		}
		activities[i] = Activity{
			Title:         title,
			UUID:          UUID,
			Screenhsot:    parseSceenshot(activity),
			SubActivities: subActivities,
		}
	}
	return activities, nil
}

func parseTestSummaries(testSummariesContent plistutil.PlistData) (*[]TestResult, error) {
	testableSummaries, err := testSummariesContent.GetMapStringInterfaceArray("TestableSummaries")
	if err != nil {
		return nil, fmt.Errorf("failed to parse test summaries plist, key TestableSummaries is not a string map")
	}

	var testResults []TestResult
	for _, testableSummariesItem := range testableSummaries {
		tests, err := testableSummariesItem.GetMapStringInterfaceArray("Tests")
		if err != nil {
			return nil, fmt.Errorf("failed to parse test summaries plist, key Tests is not a string map")
		}

		testResults = make([]TestResult, len(tests))
		for _, testsItem := range tests {
			lastSubtests, err := collectLastSubtests(testsItem)
			if err != nil {
				return nil, err
			}
			log.Printf("%s", lastSubtests)

			for i, test := range lastSubtests {
				testID, found := test.GetString("TestIdentifier")
				if !found {
					return nil, fmt.Errorf("key TestIdentifier not found for test")
				}
				testStatus, found := test.GetString("TestStatus")
				if !found {
					return nil, fmt.Errorf("key TestStatus not found for test")
				}
				var failureSummaries *[]FailureSummaries
				if testStatus == "Failure" {
					failureSummariesData, err := test.GetMapStringInterfaceArray("FailureSummaries")
					if err != nil {
						return nil, fmt.Errorf("no failure summaries found for failing test")
					}
					failureSummaries, err = parseFailureSummaries(failureSummariesData)
					if err != nil {
						return nil, fmt.Errorf("failed to parse failure summaries, error: %s", err)
					}
				}
				var activitySummaries []Activity
				{
					activitySummariesData, err := test.GetMapStringInterfaceArray("ActivitySummaries")
					if err != nil {
						return nil, fmt.Errorf("no activity summaries found for test")
					}
					activitySummaries, err = parseActivites(activitySummariesData)
					if err != nil {
						return nil, fmt.Errorf("failed to parse activities, error: %s", err)
					}
				}
				testResults[i] = TestResult{
					ID:          testID,
					TestStatus:  testStatus,
					FailureInfo: failureSummaries,
					Activities:  activitySummaries,
				}
			}
		}
	}

	return nil, nil
}

/*
// NewTestSummaries returns a TestSummaries from the provided *_TestSummaries.plist's path.
// It will search for test items with screenhot and it will set the type of the TestSummaries (TestSummariesWithScreenshotData/TestSummariesWithAttachemnts) depending on that the plist has `HasScreenshotData` fields or `Attachments` fileds.
func NewTestSummaries(testSummariesPth string) (*TestSummaries, error) {
	var testSummaries TestSummaries
	var err error

	testSummaries.Content, err = fileutil.ReadStringFromFile(testSummariesPth)
	if err != nil {
		return nil, err
	}

	testSummaries, err = testSummaries.collectTestItemsWithScreenshotAndSetType()
	if err != nil {
		return nil, err
	}

	return &testSummaries, nil
}

func (t TestSummaries) collectTestItemsWithScreenshotAndSetType() (TestSummaries, error) {
	testSummaryType := ScreenshotsLegacy

	testSummariesPlistData, err := plistutil.NewPlistDataFromContent(t.Content)
	if err != nil {
		return t, err
	}

	testableSummaries, err := testSummariesPlistData.GetMapStringInterfaceArray("TestableSummaries")
	if err != nil {
		return t, err
	}

	subActivitiesWithScreenshot := []map[string]interface{}{}
	for _, testableSummariesItem := range testableSummaries {
		tests, err := getValueAsMapStringInterfaceArray(testableSummariesItem, "Tests")
		if err != nil {
			return t, err
		}

		for _, testsItem := range tests {
			lastSubtests, err := collectLastSubtests(testsItem)
			if err != nil {
				return t, err
			}

			for _, lastSubtest := range lastSubtests {
				activitySummaries, err := getValueAsMapStringInterfaceArray(lastSubtest, "ActivitySummaries")
				if err != nil {
					continue
				}

				var subActivities []map[string]interface{}
				subActivities, testSummaryType, err = collectSubActivitiesWithScreenshots(activitySummaries)
				if err != nil {
					return t, err
				}
				subActivitiesWithScreenshot = append(subActivitiesWithScreenshot, subActivities...)
			}
		}
	}

	t.TestItemsWithScreenshots = subActivitiesWithScreenshot
	t.Type = testSummaryType
	return t, nil
}

func castToMapStringInterfaceArray(obj interface{}) ([]map[string]interface{}, error) {
	array, ok := obj.([]interface{})
	if !ok {
		return nil, errors.New("failed to cast to []interface{}")
	}

	casted := []map[string]interface{}{}
	for _, item := range array {
		mapStringInterface, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.New("failed to cast to map[string]interface{}")
		}

		casted = append(casted, mapStringInterface)
	}

	return casted, nil
}

func getValueAsMapStringInterfaceArray(obj map[string]interface{}, key string) ([]map[string]interface{}, error) {
	value, found := obj[key]
	if !found {
		return nil, fmt.Errorf("no value found for: %s", key)
	}

	return castToMapStringInterfaceArray(value)
}
*/
func collectLastSubtests(testsItem plistutil.PlistData) ([]plistutil.PlistData, error) {
	var walk func(plistutil.PlistData) []plistutil.PlistData
	walk = func(item plistutil.PlistData) []plistutil.PlistData {
		subtests, err := item.GetMapStringInterfaceArray("Subtests")
		if err != nil {
			return []plistutil.PlistData{item}
		}
		lastSubtests := []plistutil.PlistData{}
		for _, subtest := range subtests {
			last := walk(subtest)
			lastSubtests = append(lastSubtests, last...)
		}
		return lastSubtests
	}

	return walk(testsItem), nil
}

/*
func collectSubActivitiesWithScreenshots(activitySummaries []map[string]interface{}) ([]map[string]interface{}, ScreenshotsType, error) {
	testSummaryType := ScreenshotsAsAttachments

	var walk func(map[string]interface{}, *ScreenshotsType) []map[string]interface{}
	walk = func(item map[string]interface{}, summaryType *ScreenshotsType) []map[string]interface{} {
		itemWithScreenshot := []map[string]interface{}{}

		getAttachmentType := func(item map[string]interface{}) ScreenshotsType {
			value, found := item["Attachments"]
			if found {
				return ScreenshotsAsAttachments
			}
			value, found = item["HasScreenshotData"]
			if found {
				hasScreenshot, casted := value.(bool)
				if casted && hasScreenshot {
					return ScreenshotsLegacy
				}
			}
			return ScreenshotsNone
		}

		switch getAttachmentType(item) {
		case ScreenshotsAsAttachments: // New *_TestSummaries.plist
			{
				itemWithScreenshot = append(itemWithScreenshot, item)
			}
		case ScreenshotsLegacy: // Old *_TestSummaries.plist
			{
				testSummaryType = ScreenshotsLegacy
				itemWithScreenshot = append(itemWithScreenshot, item)
			}
		case ScreenshotsNone:
		}

		subActivies, err := getValueAsMapStringInterfaceArray(item, "SubActivities")
		if err == nil {
			for _, subActivity := range subActivies {
				subActivityWithScreenshots := walk(subActivity, &testSummaryType)
				itemWithScreenshot = append(itemWithScreenshot, subActivityWithScreenshots...)
			}
		}

		return itemWithScreenshot
	}

	summaries := []map[string]interface{}{}
	for _, summary := range activitySummaries {
		summaries = append(summaries, walk(summary, &testSummaryType)...)
	}

	return summaries, testSummaryType, nil
}
*/

// TimestampStrToTime ...
func TimestampStrToTime(timestampStr string) (time.Time, error) {
	timestamp, err := strconv.ParseFloat(timestampStr, 64)
	if err != nil {
		return time.Time{}, err
	}

	return TimestampToTime(timestamp)
}

// TimestampToTime ...
func TimestampToTime(timestamp float64) (time.Time, error) {
	timestampInNanosec := int64(timestamp * float64(time.Second))
	referenceDate := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	return referenceDate.Add(time.Duration(timestampInNanosec)), nil
}
