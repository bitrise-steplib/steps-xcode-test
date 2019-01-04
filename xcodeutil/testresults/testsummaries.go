package testresults

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bitrise-io/steps-xcode-test/pretty"
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
	FileName             string
	LineNumber           uint64
	Message              string
	IsPerformanceFailure bool
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
	Screenhsots   []ActivityScreenshot
	SubActivities []Activity
}

// TestResult describes a single UI test's output
type TestResult struct {
	ID          string
	TestStatus  string
	FailureInfo *[]FailureSummaries
	Activities  []Activity
}

// New parses an *_TestSummaries.plist and returns an array containing test results and screenshots
func New(testSummariesPth string) ([]TestResult, error) {
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
			return nil, fmt.Errorf("key FileName not found for FailureSummaries: %s", pretty.Object(failureSummariesData))
		}
		lineNumber, found := failureSummary.GetUInt64("LineNumber")
		if !found {
			return nil, fmt.Errorf("key lineNumber not found for FailureSummaries: %s", pretty.Object(failureSummariesData))
		}
		message, found := failureSummary.GetString("Message")
		if !found {
			return nil, fmt.Errorf("key Message not found for FailureSummaries: %s", pretty.Object(failureSummariesData))
		}
		isPerformanceFailure, found := failureSummary.GetBool("PerformanceFailure")
		if !found {
			return nil, fmt.Errorf("key PerformanceFailure not found for FailureSummaries: %s", pretty.Object(failureSummariesData))
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

func parseSceenshots(activitySummary plistutil.PlistData, activityUUID string, activityStartTime time.Time) ([]ActivityScreenshot, error) {
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

	switch getAttachmentType(activitySummary) {
	case ScreenshotsAsAttachments:
		{
			attachmentsData, err := activitySummary.GetMapStringInterfaceArray("Attachments")
			if err != nil {
				return nil, fmt.Errorf("no key Attachments, or invalid format")
			}
			attachments := make([]ActivityScreenshot, len(attachmentsData))
			for i, attachment := range attachmentsData {
				filenName, found := attachment.GetString("Filename")
				if !found {
					return nil, fmt.Errorf("no key Filename found for attachment: %s", pretty.Object(attachment))
				}
				timeStampFloat, found := attachment.GetFloat64("Timestamp")
				if !found {
					return nil, fmt.Errorf("no key Timestamp found for attachment: %s", pretty.Object(attachment))
				}
				timeStamp, err := TimestampToTime(timeStampFloat)
				if err != nil {
					return nil, fmt.Errorf("can not convert timestamp: %f, error: %s", timeStampFloat, err)
				}
				attachments[i] = ActivityScreenshot{
					FilePath:  filenName,
					Timestamp: timeStamp,
				}
			}
			return attachments, nil
		}
	case ScreenshotsLegacy:
		{
			attachments := make([]ActivityScreenshot, 2)
			for i, ext := range []string{"png", "jpg"} {
				fileName := fmt.Sprintf("Screenshot_%s.%s", activityUUID, ext)
				attachments[i] = ActivityScreenshot{
					FilePath:  fileName,
					Timestamp: activityStartTime,
				}
			}
			return attachments, nil
		}
	case ScreenshotsNone:
		{
			return nil, nil
		}
	default:
		{
			return nil, fmt.Errorf("unhandled screenshot type")
		}
	}
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
			return nil, fmt.Errorf("key UUID not found for activity: %s", activity)
		}
		timeStampFloat, found := activity.GetFloat64("StartTimeInterval")
		if !found {
			return nil, fmt.Errorf("key StartTimeInterval not found for activity: %s", activity)
		}
		timeStamp, err := TimestampToTime(timeStampFloat)
		if err != nil {
			return nil, fmt.Errorf("can not convert timestamp: %f, error: %s", timeStampFloat, err)
		}
		screenshots, err := parseSceenshots(activity, UUID, timeStamp)
		if err != nil {
			return nil, fmt.Errorf("Screenshot invalid format, error: %s", err)
		}
		var subActivities []Activity
		if subActivitiesData, err := activity.GetMapStringInterfaceArray("SubActivities"); err == nil {
			if subActivities, err = parseActivites(subActivitiesData); err != nil {
				return nil, err
			}
		}
		activities[i] = Activity{
			Title:         title,
			UUID:          UUID,
			Screenhsots:   screenshots,
			SubActivities: subActivities,
		}
	}
	return activities, nil
}

func parseTestSummaries(testSummariesContent plistutil.PlistData) ([]TestResult, error) {
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

		for _, testsItem := range tests {
			lastSubtests, err := collectLastSubtests(testsItem)
			if err != nil {
				return nil, err
			}
			log.Printf("lastSubtests %s", pretty.Object(lastSubtests))

			for _, test := range lastSubtests {
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
						log.Printf("no activity summaries found for test: %s", test)
					}
					activitySummaries, err = parseActivites(activitySummariesData)
					if err != nil {
						return nil, fmt.Errorf("failed to parse activities, error: %s", err)
					}
				}
				testResults = append(testResults, TestResult{
					ID:          testID,
					TestStatus:  testStatus,
					FailureInfo: failureSummaries,
					Activities:  activitySummaries,
				})
			}
		}
	}
	return testResults, nil
}

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
