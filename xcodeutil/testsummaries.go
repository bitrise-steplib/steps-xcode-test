package xcodeutil

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-tools/go-xcode/plistutil"
)

// ScreenshotsCategory descdribes the screenshot atttachment's type
type ScreenshotsCategory string

// const ...
const (
	ScreenshotsLegacy        ScreenshotsCategory = "ScreenshotsLegacy"
	ScreenshotsAsAttachments ScreenshotsCategory = "ScreenshotsAsAttachments"
	ScreenshotsNone          ScreenshotsCategory = "ScreenshotsNone"
)

// Type ...
type Type string

// TestSummaries ...
type TestSummaries struct {
	Type                     ScreenshotsCategory
	Content                  string
	TestItemsWithScreenshots []map[string]interface{}
}

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

	testableSummaries, err := getValueAsMapStringInterfaceArray(testSummariesPlistData, "TestableSummaries")
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

func collectLastSubtests(testsItem map[string]interface{}) ([]map[string]interface{}, error) {
	var walk func(map[string]interface{}) []map[string]interface{}
	walk = func(item map[string]interface{}) []map[string]interface{} {
		subtests, err := getValueAsMapStringInterfaceArray(item, "Subtests")
		if err != nil {
			return []map[string]interface{}{item}
		}
		lastSubtests := []map[string]interface{}{}
		for _, subtest := range subtests {
			last := walk(subtest)
			lastSubtests = append(lastSubtests, last...)
		}
		return lastSubtests
	}

	return walk(testsItem), nil
}

func collectSubActivitiesWithScreenshots(activitySummaries []map[string]interface{}) ([]map[string]interface{}, ScreenshotsCategory, error) {
	testSummaryType := ScreenshotsAsAttachments

	var walk func(map[string]interface{}, *ScreenshotsCategory) []map[string]interface{}
	walk = func(item map[string]interface{}, summaryType *ScreenshotsCategory) []map[string]interface{} {
		itemWithScreenshot := []map[string]interface{}{}

		getAttachmentType := func(item map[string]interface{}) ScreenshotsCategory {
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
